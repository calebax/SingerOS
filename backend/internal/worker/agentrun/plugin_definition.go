package agentrun

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/insmtx/Leros/backend/agent"
	"github.com/insmtx/Leros/backend/internal/service"
	"github.com/insmtx/Leros/backend/internal/worker/agentrun/domain"
)

// MCPServerConfigFromPluginSnapshot decodes a validated MCP snapshot.
func MCPServerConfigFromPluginSnapshot(snapshot domain.PluginSnapshot) (agent.MCPServerConfig, error) {
	if !strings.EqualFold(snapshot.Kind, "mcp") {
		return agent.MCPServerConfig{}, fmt.Errorf("snapshot kind is not mcp")
	}
	code := strings.ToLower(strings.TrimSpace(snapshot.Code))
	if code == "" {
		return agent.MCPServerConfig{}, fmt.Errorf("mcp snapshot code is required")
	}
	if code == "leros" {
		return agent.MCPServerConfig{}, fmt.Errorf("mcp snapshot uses a reserved code")
	}
	definition, err := service.MCPFromDefinition(snapshot.Definition)
	if err != nil {
		return agent.MCPServerConfig{}, err
	}
	switch definition.Transport {
	case "http":
		parsed, parseErr := url.Parse(strings.TrimSpace(definition.URL))
		if parseErr != nil || parsed == nil || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
			return agent.MCPServerConfig{}, fmt.Errorf("mcp snapshot has an invalid HTTP URL")
		}
	case "stdio":
	default:
		return agent.MCPServerConfig{}, fmt.Errorf("mcp snapshot transport is unsupported")
	}
	return agent.MCPServerConfig{
		Name:        code,
		URL:         definition.URL,
		Command:     definition.Command,
		Args:        append([]string(nil), definition.Args...),
		Env:         cloneMCPStringMap(definition.Env),
		Headers:     cloneMCPStringMap(definition.Headers),
		BearerToken: definition.BearerToken,
	}, nil
}

func cloneMCPStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
