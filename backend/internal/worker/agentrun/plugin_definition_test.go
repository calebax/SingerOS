package agentrun

import (
	"encoding/json"
	"testing"

	"github.com/insmtx/Leros/backend/internal/worker/agentrun/domain"
)

func TestMCPServerConfigFromPluginSnapshot(t *testing.T) {
	config, err := MCPServerConfigFromPluginSnapshot(domain.PluginSnapshot{
		PluginID: "plg_mcp",
		Code:     "docs",
		Kind:     "mcp",
		Definition: json.RawMessage(
			`{"schema":"mcp/v1","transport":"http","name":"ignored","url":"https://example.com/mcp","bearer_token":"runtime-secret","headers":{"X-Tenant":"docs"}}`,
		),
	})
	if err != nil || config.Name != "docs" || config.URL != "https://example.com/mcp" ||
		config.BearerToken != "runtime-secret" || config.Headers["X-Tenant"] != "docs" {
		t.Fatalf("config=%#v err=%v", config, err)
	}
}

func TestPrepareMCPServersSortsAndSkipsInvalidSnapshots(t *testing.T) {
	configs := prepareMCPServers(t.Context(), []domain.PluginSnapshot{
		{
			PluginID: "plg_z",
			Code:     "zeta",
			Kind:     "mcp",
			Definition: json.RawMessage(
				`{"schema":"mcp/v1","transport":"http","name":"zeta","url":"https://z.example.com/mcp"}`,
			),
		},
		{
			PluginID: "plg_invalid",
			Code:     "invalid",
			Kind:     "mcp",
			Definition: json.RawMessage(
				`{"schema":"mcp/v1","transport":"http","name":"invalid","url":"file:///tmp/mcp"}`,
			),
		},
		{
			PluginID: "plg_a",
			Code:     "alpha",
			Kind:     "mcp",
			Definition: json.RawMessage(
				`{"schema":"mcp/v1","transport":"http","name":"alpha","url":"https://a.example.com/mcp"}`,
			),
		},
	})
	if len(configs) != 2 || configs[0].Name != "alpha" || configs[1].Name != "zeta" {
		t.Fatalf("configs = %#v", configs)
	}
}
