package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/insmtx/SingerOS/backend/tools"
)

type mockTool struct {
	info *tools.ToolInfo
}

func (m *mockTool) Info() *tools.ToolInfo {
	return m.info
}

func (m *mockTool) Validate(input map[string]interface{}) error {
	return nil
}

func (m *mockTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func TestBuildToolsContext(t *testing.T) {
	registry := tools.NewRegistry()

	if err := registry.Register(&mockTool{
		info: &tools.ToolInfo{
			Name:        "github.pr.get_metadata",
			Description: "Read pull request metadata",
			Provider:    "github",
			ReadOnly:    true,
			InputSchema: &tools.Schema{
				Type:     "object",
				Required: []string{"repo", "pr_number"},
			},
		},
	}); err != nil {
		t.Fatalf("register read tool: %v", err)
	}

	if err := registry.Register(&mockTool{
		info: &tools.ToolInfo{
			Name:        "github.pr.publish_review",
			Description: "Publish a pull request review",
			Provider:    "github",
			ReadOnly:    false,
			InputSchema: &tools.Schema{
				Type:     "object",
				Required: []string{"repo", "pr_number", "body"},
			},
		},
	}); err != nil {
		t.Fatalf("register write tool: %v", err)
	}

	context := BuildToolsContext(registry)
	if context == nil {
		t.Fatalf("expected non-nil tools context")
	}
	if !strings.Contains(context.SummarySection, "github.pr.get_metadata: Read pull request metadata") {
		t.Fatalf("missing read tool summary: %s", context.SummarySection)
	}
	if !strings.Contains(context.SummarySection, "[mode=read]") {
		t.Fatalf("missing read mode marker: %s", context.SummarySection)
	}
	if !strings.Contains(context.SummarySection, "github.pr.publish_review: Publish a pull request review") {
		t.Fatalf("missing write tool summary: %s", context.SummarySection)
	}
	if !strings.Contains(context.SummarySection, "[mode=write]") {
		t.Fatalf("missing write mode marker: %s", context.SummarySection)
	}
	if !strings.Contains(context.SummarySection, "[required=repo,pr_number,body]") {
		t.Fatalf("missing required fields summary: %s", context.SummarySection)
	}
}

func TestBuildToolsContextForNames(t *testing.T) {
	registry := tools.NewRegistry()
	if err := registry.Register(&mockTool{
		info: &tools.ToolInfo{
			Name:        "node_shell",
			Description: "Execute shell command",
			Provider:    "node",
			ReadOnly:    false,
		},
	}); err != nil {
		t.Fatalf("register node shell tool: %v", err)
	}
	if err := registry.Register(&mockTool{
		info: &tools.ToolInfo{
			Name:        "node_file_read",
			Description: "Read file",
			Provider:    "node",
			ReadOnly:    true,
		},
	}); err != nil {
		t.Fatalf("register node file read tool: %v", err)
	}

	context, err := BuildToolsContextForNames(registry, []string{"node_file_read"})
	if err != nil {
		t.Fatalf("build filtered tools context: %v", err)
	}
	if context == nil {
		t.Fatalf("expected non-nil tools context")
	}
	if strings.Contains(context.SummarySection, "node_shell") {
		t.Fatalf("unexpected unselected tool in summary: %s", context.SummarySection)
	}
	if !strings.Contains(context.SummarySection, "node_file_read: Read file") {
		t.Fatalf("missing selected tool summary: %s", context.SummarySection)
	}
}
