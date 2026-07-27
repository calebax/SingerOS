package agentrun

import (
	"encoding/json"
	"testing"

	"github.com/insmtx/Leros/backend/internal/worker/agentrun/domain"
)

func TestMCPServerConfigFromPluginSnapshot(t *testing.T) {
	config, err := MCPServerConfigFromPluginSnapshot(domain.PluginSnapshot{PluginID: "plg_mcp", Kind: "mcp", Definition: json.RawMessage(`{"schema":"mcp/v1","transport":"stdio","name":"demo","command":"demo-mcp","args":["--stdio"]}`)})
	if err != nil || config.Command != "demo-mcp" || len(config.Args) != 1 {
		t.Fatalf("config=%#v err=%v", config, err)
	}
}
