package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ygpkg/yg-go/logs"

	"github.com/insmtx/Leros/backend/internal/adapter/account"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

const coreKGPlatformCode = "corekg"

func (s *pluginService) ListMCPPlatforms(
	ctx context.Context,
	orgID, uin uint,
) (*contract.ListMCPPlatformsResponse, error) {
	if orgID == 0 || uin == 0 {
		return nil, invalidMCPConfig("organization and user identity are required")
	}
	channels, err := infradb.ListActiveMCPChannels(ctx, s.db)
	if err != nil {
		return nil, err
	}
	platforms := make([]contract.MCPPlatformView, 0, len(channels))
	for index := range channels {
		channel, ok := normalizeSupportedMCPChannel(&channels[index])
		if !ok {
			continue
		}
		platform, viewErr := s.mcpPlatformView(ctx, orgID, uin, channel)
		if viewErr != nil {
			return nil, viewErr
		}
		platforms = append(platforms, platform)
	}
	return &contract.ListMCPPlatformsResponse{Platforms: platforms}, nil
}

func (s *pluginService) ConnectMCPPlatform(
	ctx context.Context,
	orgID, uin uint,
	platformCode string,
) (*contract.ConnectMCPPlatformResponse, error) {
	if orgID == 0 || uin == 0 {
		return nil, invalidMCPConfig("organization and user identity are required")
	}
	channelCode := strings.ToLower(strings.TrimSpace(platformCode))
	if channelCode != coreKGPlatformCode {
		return nil, invalidMCPConfig("unsupported MCP platform")
	}
	channel, err := s.getSupportedMCPChannel(ctx, channelCode)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, invalidMCPConfig("MCP platform is not configured or active")
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
		platform := mcpPlatformViewFromPlugin(channel, s.apiKeyIssuer != nil, existing.PublicID)
		view := pluginView(*existing)
		return &contract.ConnectMCPPlatformResponse{Platform: platform, Plugin: view}, nil
	}
	if s.apiKeyIssuer == nil {
		return nil, invalidMCPConfig("current edition does not support CoreKG automatic authorization")
	}

	credential, err := s.apiKeyIssuer.CreateAPIKey(ctx, account.CreateAPIKeyInput{
		Name:         "SingerOS " + channel.Name + " MCP",
		Purpose:      "mcp_connector",
		ResourceType: "mcp",
		ResourceID:   0,
		ExpireHours:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("create CoreKG API key: %w", err)
	}
	headers := cloneStringMap(map[string]string(channel.Headers))
	testResult, err := s.TestMCPPlugin(ctx, &contract.TestMCPPluginRequest{
		Transport:   channel.Transport,
		URL:         channel.URL,
		BearerToken: credential.APIKey,
		Headers:     headers,
	})
	if err != nil {
		return nil, err
	}
	created, err := s.AddMCPPlugin(ctx, orgID, uin, &contract.AddMCPPluginRequest{
		MCPPluginConfig: contract.MCPPluginConfig{
			Code:        code,
			Name:        channel.Name,
			Description: channel.Description,
			Transport:   channel.Transport,
			URL:         channel.URL,
			BearerToken: credential.APIKey,
			Headers:     headers,
			Provider:    channel.Channel,
		},
	})
	if err != nil {
		if existing, lookupErr := infradb.GetOrganizationPluginByIdentity(ctx, s.db, orgID, "mcp", code); lookupErr == nil &&
			existing != nil && existing.CreatedBy == uin {
			platform := mcpPlatformViewFromPlugin(channel, true, existing.PublicID)
			view := pluginView(*existing)
			return &contract.ConnectMCPPlatformResponse{
				Platform:  platform,
				Plugin:    view,
				ToolCount: testResult.ToolCount,
			}, nil
		}
		return nil, err
	}
	platform := mcpPlatformViewFromPlugin(channel, true, created.PublicID)
	return &contract.ConnectMCPPlatformResponse{
		Platform:  platform,
		Plugin:    *created,
		ToolCount: testResult.ToolCount,
	}, nil
}

func (s *pluginService) getSupportedMCPChannel(
	ctx context.Context,
	channelCode string,
) (*types.MCPChannel, error) {
	channel, err := infradb.GetActiveMCPChannelByChannel(ctx, s.db, channelCode)
	if err != nil || channel == nil {
		return channel, err
	}
	normalized, ok := normalizeSupportedMCPChannel(channel)
	if !ok {
		return nil, nil
	}
	return normalized, nil
}

func normalizeSupportedMCPChannel(channel *types.MCPChannel) (*types.MCPChannel, bool) {
	if channel == nil {
		return nil, false
	}
	normalized := *channel
	normalized.Channel = strings.TrimSpace(channel.Channel)
	normalized.Name = strings.TrimSpace(channel.Name)
	normalized.Description = strings.TrimSpace(channel.Description)
	normalized.Transport = strings.ToLower(strings.TrimSpace(channel.Transport))
	normalized.URL = strings.TrimSpace(channel.URL)
	normalized.Headers = cloneMCPChannelHeaders(channel.Headers)

	reason := ""
	switch {
	case channel.Channel != normalized.Channel || normalized.Channel != strings.ToLower(normalized.Channel):
		reason = "channel must be lowercase without surrounding whitespace"
	case normalized.Channel != coreKGPlatformCode:
		reason = "unsupported channel"
	case normalized.Name == "":
		reason = "name is required"
	case normalized.Transport != "http":
		reason = "transport must be http"
	case hasMCPHeader(map[string]string(normalized.Headers), "authorization"):
		reason = "Authorization header is not allowed"
	default:
		if err := validateMCPConnection(normalized.URL, map[string]string(normalized.Headers)); err != nil {
			reason = err.Error()
		}
	}
	if reason != "" {
		logs.Warnf("Skipping invalid MCP channel: id=%d channel=%q reason=%s", channel.ID, channel.Channel, reason)
		return nil, false
	}
	return &normalized, true
}

func cloneMCPChannelHeaders(headers types.MCPChannelHeaders) types.MCPChannelHeaders {
	if len(headers) == 0 {
		return types.MCPChannelHeaders{}
	}
	result := make(types.MCPChannelHeaders, len(headers))
	for key, value := range headers {
		result[key] = value
	}
	return result
}

func (s *pluginService) mcpPlatformView(
	ctx context.Context,
	orgID, uin uint,
	channel *types.MCPChannel,
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
		return mcpPlatformViewFromPlugin(channel, s.apiKeyIssuer != nil, ""), nil
	}
	return mcpPlatformViewFromPlugin(channel, s.apiKeyIssuer != nil, plugin.PublicID), nil
}

func mcpPlatformViewFromPlugin(
	channel *types.MCPChannel,
	autoConnectSupported bool,
	pluginID string,
) contract.MCPPlatformView {
	return contract.MCPPlatformView{
		Code:                 channel.Channel,
		Name:                 channel.Name,
		Description:          channel.Description,
		AutoConnectSupported: autoConnectSupported,
		Connected:            pluginID != "",
		PluginID:             pluginID,
	}
}

func coreKGPluginCode(orgID, uin uint) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", orgID, uin, coreKGPlatformCode)))
	return coreKGPlatformCode + "-" + hex.EncodeToString(sum[:16])
}
