package agentrun

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/insmtx/Leros/backend/agent"
	"github.com/insmtx/Leros/backend/internal/service"
	"github.com/insmtx/Leros/backend/internal/worker/agentrun/domain"
)

type mcpPluginDefinition struct {
	Schema    string   `json:"schema"`
	Transport string   `json:"transport"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
}

// MCPServerConfigFromPluginSnapshot decodes a validated MCP snapshot.
func MCPServerConfigFromPluginSnapshot(snapshot domain.PluginSnapshot) (agent.MCPServerConfig, error) {
	if !strings.EqualFold(snapshot.Kind, "mcp") {
		return agent.MCPServerConfig{}, fmt.Errorf("plugin %s is not an mcp plugin", snapshot.PluginID)
	}
	if err := service.ValidatePluginDefinition(snapshot.Kind, snapshot.Definition); err != nil {
		return agent.MCPServerConfig{}, err
	}
	var definition mcpPluginDefinition
	if err := json.Unmarshal(snapshot.Definition, &definition); err != nil {
		return agent.MCPServerConfig{}, err
	}
	return agent.MCPServerConfig{Name: definition.Name, URL: definition.URL, Command: definition.Command, Args: append([]string(nil), definition.Args...)}, nil
}
