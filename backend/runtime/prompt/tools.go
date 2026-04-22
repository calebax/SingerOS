package prompt

import (
	"fmt"
	"strings"

	"github.com/insmtx/SingerOS/backend/tools"
)

// ToolsContext is the prompt-ready projection of the runtime tool registry.
type ToolsContext struct {
	SummarySection string
}

// BuildToolsContext converts a tool registry into a compact summary for runtime prompts.
func BuildToolsContext(registry *tools.Registry) *ToolsContext {
	if registry == nil {
		return &ToolsContext{}
	}

	infos := registry.ListInfos()
	if len(infos) == 0 {
		return &ToolsContext{}
	}

	return &ToolsContext{
		SummarySection: buildToolsSummary(infos),
	}
}

// BuildToolsContextForNames converts selected tools into a prompt-ready summary.
func BuildToolsContextForNames(registry *tools.Registry, names []string) (*ToolsContext, error) {
	if registry == nil {
		return &ToolsContext{}, nil
	}
	if len(names) == 0 {
		return BuildToolsContext(registry), nil
	}

	infos := make([]tools.ToolInfo, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}

		tool, err := registry.Get(name)
		if err != nil {
			return nil, err
		}
		info := tool.Info()
		if info == nil {
			return nil, fmt.Errorf("tool %s info is required", name)
		}
		infos = append(infos, *info)
	}
	if len(infos) == 0 {
		return &ToolsContext{}, nil
	}

	return &ToolsContext{
		SummarySection: buildToolsSummary(infos),
	}, nil
}

func buildToolsSummary(infos []tools.ToolInfo) string {
	var builder strings.Builder

	builder.WriteString("Available tools:\n")
	for _, info := range infos {
		builder.WriteString("- ")
		builder.WriteString(info.Name)
		builder.WriteString(": ")
		builder.WriteString(info.Description)
		if info.Provider != "" {
			builder.WriteString(" [provider=")
			builder.WriteString(info.Provider)
			builder.WriteString("]")
		}
		if info.ReadOnly {
			builder.WriteString(" [mode=read]")
		} else {
			builder.WriteString(" [mode=write]")
		}
		if info.InputSchema != nil && len(info.InputSchema.Required) > 0 {
			builder.WriteString(" [required=")
			builder.WriteString(strings.Join(info.InputSchema.Required, ","))
			builder.WriteString("]")
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\nUse read tools first to gather context before calling write tools.")
	return strings.TrimSpace(builder.String())
}
