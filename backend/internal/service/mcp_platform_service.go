package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/insmtx/Leros/backend/internal/adapter/account"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
)

const coreKGPlatformCode = "corekg"

func (s *pluginService) ListMCPPlatforms(
	ctx context.Context,
	orgID, uin uint,
) (*contract.ListMCPPlatformsResponse, error) {
	if orgID == 0 || uin == 0 {
		return nil, invalidMCPConfig("organization and user identity are required")
	}
	platform, err := s.coreKGPlatformView(ctx, orgID, uin)
	if err != nil {
		return nil, err
	}
	return &contract.ListMCPPlatformsResponse{Platforms: []contract.MCPPlatformView{platform}}, nil
}

func (s *pluginService) ConnectMCPPlatform(
	ctx context.Context,
	orgID, uin uint,
	platformCode string,
) (*contract.ConnectMCPPlatformResponse, error) {
	if orgID == 0 || uin == 0 {
		return nil, invalidMCPConfig("organization and user identity are required")
	}
	if strings.ToLower(strings.TrimSpace(platformCode)) != coreKGPlatformCode {
		return nil, invalidMCPConfig("unsupported MCP platform")
	}

	code := coreKGPluginCode(orgID, uin)
	existing, err := infradb.GetOrganizationPluginByIdentity(ctx, s.db, orgID, "mcp", code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.CreatedBy != uin {
			return nil, contract.ErrPluginNotFound
		}
		platform := coreKGPlatformViewFromPlugin(s.coreKGAutoConnectSupported(), existing.PublicID)
		view := pluginView(*existing)
		return &contract.ConnectMCPPlatformResponse{Platform: platform, Plugin: view}, nil
	}
	if s.apiKeyIssuer == nil {
		return nil, invalidMCPConfig("current edition does not support CoreKG automatic authorization")
	}
	coreKGURL := s.coreKGURL()
	if coreKGURL == "" {
		return nil, invalidMCPConfig("auth base URL is required for CoreKG automatic authorization")
	}

	credential, err := s.apiKeyIssuer.CreateAPIKey(ctx, account.CreateAPIKeyInput{
		Name:         "SingerOS CoreKG MCP",
		Purpose:      "mcp_connector",
		ResourceType: "mcp",
		ResourceID:   0,
		ExpireHours:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("create CoreKG API key: %w", err)
	}
	testResult, err := s.TestMCPPlugin(ctx, &contract.TestMCPPluginRequest{
		URL:         coreKGURL,
		BearerToken: credential.APIKey,
	})
	if err != nil {
		return nil, err
	}
	created, err := s.AddMCPPlugin(ctx, orgID, uin, &contract.AddMCPPluginRequest{
		MCPPluginConfig: contract.MCPPluginConfig{
			Code:        code,
			Name:        "CoreKG",
			Description: "连接 CoreKG 知识库，使用知识检索、图谱与问答能力。",
			URL:         coreKGURL,
			BearerToken: credential.APIKey,
			Provider:    coreKGPlatformCode,
		},
	})
	if err != nil {
		if existing, lookupErr := infradb.GetOrganizationPluginByIdentity(ctx, s.db, orgID, "mcp", code); lookupErr == nil &&
			existing != nil && existing.CreatedBy == uin {
			platform := coreKGPlatformViewFromPlugin(true, existing.PublicID)
			view := pluginView(*existing)
			return &contract.ConnectMCPPlatformResponse{
				Platform:  platform,
				Plugin:    view,
				ToolCount: testResult.ToolCount,
			}, nil
		}
		return nil, err
	}
	platform := coreKGPlatformViewFromPlugin(true, created.PublicID)
	return &contract.ConnectMCPPlatformResponse{
		Platform:  platform,
		Plugin:    *created,
		ToolCount: testResult.ToolCount,
	}, nil
}

func (s *pluginService) coreKGURL() string {
	return strings.TrimSpace(s.coreKGMCPURL)
}

func coreKGMCPURLFromAuthBase(authBaseURL string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(authBaseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/v3/keapi/mcp"
}

func (s *pluginService) coreKGPlatformView(
	ctx context.Context,
	orgID, uin uint,
) (contract.MCPPlatformView, error) {
	plugin, err := infradb.GetOrganizationPluginByIdentity(
		ctx,
		s.db,
		orgID,
		"mcp",
		coreKGPluginCode(orgID, uin),
	)
	if err != nil {
		return contract.MCPPlatformView{}, err
	}
	if plugin == nil || plugin.CreatedBy != uin {
		return coreKGPlatformViewFromPlugin(s.coreKGAutoConnectSupported(), ""), nil
	}
	return coreKGPlatformViewFromPlugin(s.coreKGAutoConnectSupported(), plugin.PublicID), nil
}

func (s *pluginService) coreKGAutoConnectSupported() bool {
	return s.apiKeyIssuer != nil && s.coreKGURL() != ""
}

func coreKGPlatformViewFromPlugin(autoConnectSupported bool, pluginID string) contract.MCPPlatformView {
	return contract.MCPPlatformView{
		Code:                 coreKGPlatformCode,
		Name:                 "CoreKG",
		Description:          "连接知识库、知识图谱与智能问答能力",
		AutoConnectSupported: autoConnectSupported,
		Connected:            pluginID != "",
		PluginID:             pluginID,
	}
}

func coreKGPluginCode(orgID, uin uint) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", orgID, uin, coreKGPlatformCode)))
	return coreKGPlatformCode + "-" + hex.EncodeToString(sum[:16])
}
