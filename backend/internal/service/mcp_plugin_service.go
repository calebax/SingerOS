package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

const (
	maxMCPDefinitionBytes = 64 * 1024
	mcpTestTimeout        = 10 * time.Second
)

var (
	mcpCodePattern    = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)
	headerNamePattern = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)
	reservedMCPNames  = map[string]struct{}{
		"leros": {},
	}
)

func (s *pluginService) AddMCPPlugin(
	ctx context.Context,
	orgID, uin uint,
	req *contract.AddMCPPluginRequest,
) (*contract.PluginView, error) {
	if orgID == 0 || uin == 0 {
		return nil, invalidMCPConfig("organization and user identity are required")
	}
	if req == nil {
		return nil, invalidMCPConfig("request is required")
	}
	input := req.MCPPluginConfig
	if strings.TrimSpace(input.Code) == "" {
		input.Code = "mcp-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	config, definition, err := validateMCPPluginConfig(input)
	if err != nil {
		return nil, err
	}

	var created types.Plugin
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := infradb.GetOrganizationPluginByIdentity(ctx, tx, orgID, "mcp", config.Code)
		if err != nil {
			return err
		}
		if existing != nil {
			return invalidMCPConfig("MCP code already exists")
		}

		created = types.Plugin{
			PublicID:        "plugin_" + uuid.NewString(),
			OwnerScope:      types.OwnerScopeOrganization,
			OrgID:           orgID,
			Code:            config.Code,
			Kind:            "mcp",
			Name:            config.Name,
			Description:     config.Description,
			Status:          types.PluginStatusActive,
			Origin:          "org",
			CurrentRevision: 0,
			CreatedBy:       uin,
			UpdatedBy:       uin,
		}
		if err := infradb.CreatePlugin(ctx, tx, &created); err != nil {
			if isPluginIdentityConflict(err) {
				return invalidMCPConfig("MCP code already exists")
			}
			return err
		}
		revision := &types.PluginRevision{
			PluginID:        created.ID,
			Revision:        1,
			Status:          "published",
			Definition:      definition,
			PublishedByType: "user",
			PublishedByID:   uin,
			PublishedAt:     time.Now(),
		}
		if err := infradb.CreatePluginRevision(ctx, tx, revision); err != nil {
			return err
		}
		if err := tx.Model(&types.Plugin{}).Where("id = ?", created.ID).
			Select("current_revision", "updated_by").
			Updates(types.Plugin{CurrentRevision: 1, UpdatedBy: uin}).Error; err != nil {
			return err
		}
		created.CurrentRevision = 1
		return nil
	})
	if err != nil {
		return nil, err
	}
	view := pluginView(created)
	return &view, nil
}

func (s *pluginService) UpdateMCPPlugin(
	ctx context.Context,
	orgID, uin uint,
	pluginID string,
	req *contract.UpdateMCPPluginRequest,
) (*contract.PluginView, error) {
	if orgID == 0 || uin == 0 {
		return nil, invalidMCPConfig("organization and user identity are required")
	}
	if strings.TrimSpace(pluginID) == "" {
		return nil, invalidMCPConfig("plugin_id is required")
	}
	if req == nil {
		return nil, invalidMCPConfig("request is required")
	}

	var updated types.Plugin
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(
				"owner_scope = ? AND org_id = ? AND public_id = ? AND created_by = ?",
				types.OwnerScopeOrganization,
				orgID,
				pluginID,
				uin,
			).
			First(&updated).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return contract.ErrPluginNotFound
		}
		if err != nil {
			return err
		}
		if updated.Kind != "mcp" {
			return invalidMCPConfig("plugin kind must be mcp")
		}
		if updated.Status != types.PluginStatusActive {
			return invalidMCPConfig("MCP plugin must be active")
		}

		input := req.MCPPluginConfig
		if strings.TrimSpace(input.Code) == "" {
			input.Code = updated.Code
		}
		if strings.TrimSpace(input.Description) == "" {
			input.Description = updated.Description
		}
		currentRevision, currentErr := infradb.GetCurrentPluginRevision(ctx, tx, &updated)
		if currentErr != nil {
			return currentErr
		}
		if currentRevision != nil {
			currentDefinition, definitionErr := MCPFromDefinition(currentRevision.Definition)
			if definitionErr != nil {
				return definitionErr
			}
			input.Provider = currentDefinition.Provider
		}
		config, definition, validateErr := validateMCPPluginConfig(input)
		if validateErr != nil {
			return validateErr
		}
		if config.Code != updated.Code {
			return invalidMCPConfig("MCP code cannot be changed")
		}

		nextRevision := updated.CurrentRevision + 1
		if nextRevision <= 0 {
			nextRevision = 1
		}
		revision := &types.PluginRevision{
			PluginID:        updated.ID,
			Revision:        nextRevision,
			Status:          "published",
			Definition:      definition,
			PublishedByType: "user",
			PublishedByID:   uin,
			PublishedAt:     time.Now(),
		}
		if err := infradb.CreatePluginRevision(ctx, tx, revision); err != nil {
			return err
		}
		if err := tx.Model(&types.Plugin{}).Where("id = ?", updated.ID).
			Select("name", "description", "current_revision", "updated_by").
			Updates(types.Plugin{
				Name:            config.Name,
				Description:     config.Description,
				CurrentRevision: nextRevision,
				UpdatedBy:       uin,
			}).Error; err != nil {
			return err
		}
		updated.Name = config.Name
		updated.Description = config.Description
		updated.CurrentRevision = nextRevision
		updated.UpdatedBy = uin
		return nil
	})
	if err != nil {
		return nil, err
	}
	view := pluginView(updated)
	return &view, nil
}

func (s *pluginService) TestMCPPlugin(
	ctx context.Context,
	req *contract.TestMCPPluginRequest,
) (*contract.TestMCPPluginResponse, error) {
	if req == nil {
		return nil, invalidMCPConfig("request is required")
	}
	if err := validateMCPConnection(req.URL, req.Headers); err != nil {
		return nil, err
	}
	headers := cloneStringMap(req.Headers)
	token := strings.TrimSpace(req.BearerToken)
	if strings.ContainsAny(token, "\r\n") {
		return nil, invalidMCPConfig("Bearer token contains invalid characters")
	}
	if token != "" && !hasMCPHeader(headers, "authorization") {
		if headers == nil {
			headers = make(map[string]string)
		}
		headers["Authorization"] = "Bearer " + token
	}
	draft, err := json.Marshal(MCPDefinition{
		Schema: "mcp/v1", Transport: "http", Name: "connection-test",
		URL: strings.TrimSpace(req.URL), Headers: headers,
	})
	if err != nil || len(draft) > maxMCPDefinitionBytes {
		return nil, invalidMCPConfig("MCP definition is too large")
	}

	testCtx, cancel := context.WithTimeout(ctx, mcpTestTimeout)
	defer cancel()
	client, err := mcpclient.NewStreamableHttpClient(
		strings.TrimSpace(req.URL),
		mcptransport.WithHTTPHeaders(headers),
		mcptransport.WithHTTPTimeout(mcpTestTimeout),
	)
	if err != nil {
		return nil, invalidMCPConfig("unable to create MCP client")
	}
	defer client.Close()
	if err := client.Start(testCtx); err != nil {
		return nil, invalidMCPConfig("MCP connection failed")
	}
	initialize := mcpsdk.InitializeRequest{}
	initialize.Params.ProtocolVersion = mcpsdk.LATEST_PROTOCOL_VERSION
	initialize.Params.ClientInfo = mcpsdk.Implementation{Name: "Leros MCP Test", Version: "1.0"}
	initialize.Params.Capabilities = mcpsdk.ClientCapabilities{}
	if _, err := client.Initialize(testCtx, initialize); err != nil {
		return nil, invalidMCPConfig("MCP initialize failed")
	}
	tools, err := client.ListTools(testCtx, mcpsdk.ListToolsRequest{})
	if err != nil {
		return nil, invalidMCPConfig("MCP tools/list failed")
	}
	return &contract.TestMCPPluginResponse{OK: true, ToolCount: len(tools.Tools)}, nil
}

func validateMCPPluginConfig(input contract.MCPPluginConfig) (contract.MCPPluginConfig, json.RawMessage, error) {
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	input.URL = strings.TrimSpace(input.URL)
	input.BearerToken = strings.TrimSpace(input.BearerToken)
	if strings.ContainsAny(input.BearerToken, "\r\n") {
		return input, nil, invalidMCPConfig("Bearer token contains invalid characters")
	}
	if !mcpCodePattern.MatchString(input.Code) {
		return input, nil, invalidMCPConfig("code must be a lowercase slug of at most 64 characters")
	}
	if _, reserved := reservedMCPNames[input.Code]; reserved {
		return input, nil, invalidMCPConfig("MCP code is reserved")
	}
	if input.Name == "" {
		return input, nil, invalidMCPConfig("name is required")
	}
	if len(input.Name) > 255 {
		return input, nil, invalidMCPConfig("name is too long")
	}
	if err := validateMCPConnection(input.URL, input.Headers); err != nil {
		return input, nil, err
	}
	definition, err := json.Marshal(MCPDefinition{
		Schema:      "mcp/v1",
		Transport:   "http",
		Name:        input.Code,
		Provider:    input.Provider,
		URL:         input.URL,
		BearerToken: input.BearerToken,
		Headers:     cloneStringMap(input.Headers),
	})
	if err != nil {
		return input, nil, invalidMCPConfig("unable to encode MCP definition")
	}
	if len(definition) > maxMCPDefinitionBytes {
		return input, nil, invalidMCPConfig("MCP definition is too large")
	}
	if err := ValidatePluginDefinition("mcp", definition); err != nil {
		return input, nil, invalidMCPConfig("invalid MCP definition")
	}
	return input, definition, nil
}

func validateMCPConnection(rawURL string, headers map[string]string) error {
	value, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || value == nil || value.Host == "" ||
		(value.Scheme != "http" && value.Scheme != "https") || value.User != nil {
		return invalidMCPConfig("URL must be an absolute HTTP or HTTPS URL without user info")
	}
	seenHeaders := make(map[string]struct{}, len(headers))
	for name, value := range headers {
		trimmedName := strings.TrimSpace(name)
		if name != trimmedName || !headerNamePattern.MatchString(trimmedName) {
			return invalidMCPConfig("header name is invalid")
		}
		normalizedName := strings.ToLower(trimmedName)
		if _, exists := seenHeaders[normalizedName]; exists {
			return invalidMCPConfig("header name is duplicated")
		}
		seenHeaders[normalizedName] = struct{}{}
		if strings.ContainsAny(value, "\r\n") {
			return invalidMCPConfig("header value contains invalid characters")
		}
	}
	return nil
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func hasMCPHeader(headers map[string]string, name string) bool {
	for key := range headers {
		if strings.EqualFold(strings.TrimSpace(key), name) {
			return true
		}
	}
	return false
}

func invalidMCPConfig(message string) error {
	return fmt.Errorf("%w: %s", contract.ErrInvalidPluginConfig, message)
}

func isPluginIdentityConflict(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}
