package nodetools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/insmtx/SingerOS/backend/tools"
)

// NodeFileWriteTool writes files to a node container.
type NodeFileWriteTool struct {
	executor nodeExecutor
}

// NewNodeFileWriteTool creates a Docker-backed node file write tool.
func NewNodeFileWriteTool() *NodeFileWriteTool {
	return newNodeFileWriteToolWithExecutor(dockerCLIExecutor{})
}

func newNodeFileWriteToolWithExecutor(executor nodeExecutor) *NodeFileWriteTool {
	return &NodeFileWriteTool{executor: executor}
}

// Info returns metadata for the node file write tool.
func (t *NodeFileWriteTool) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        ToolNameNodeFileWrite,
		Description: "Create or modify a file inside an assistant node Docker container",
		Provider:    ProviderNode,
		ReadOnly:    false,
		InputSchema: &tools.Schema{
			Type:     "object",
			Required: []string{"container_id", "path", "content"},
			Properties: map[string]*tools.Property{
				"container_id": {
					Type:        "string",
					Description: "Docker container id for the assistant node",
				},
				"path": {
					Type:        "string",
					Description: "File path inside the container",
				},
				"content": {
					Type:        "string",
					Description: "File content to write",
				},
				"append": {
					Type:        "boolean",
					Description: "Append to the file instead of overwriting it",
				},
			},
		},
	}
}

// Validate checks node file write tool input.
func (t *NodeFileWriteTool) Validate(input map[string]interface{}) error {
	if input == nil {
		return fmt.Errorf("input is required")
	}
	if stringValue(input, "container_id") == "" {
		return fmt.Errorf("container_id is required")
	}
	if stringValue(input, "path") == "" {
		return fmt.Errorf("path is required")
	}
	if _, ok := input["content"].(string); !ok {
		return fmt.Errorf("content is required")
	}
	if _, err := boolValue(input["append"]); err != nil {
		return fmt.Errorf("append must be a boolean")
	}
	return nil
}

// Execute writes a file to the target node container.
func (t *NodeFileWriteTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if err := t.Validate(input); err != nil {
		return nil, err
	}
	if t.executor == nil {
		return nil, fmt.Errorf("node executor is required")
	}

	containerID := stringValue(input, "container_id")
	path := stringValue(input, "path")
	content := input["content"].(string)
	appendMode, _ := boolValue(input["append"])

	if dir := parentDir(path); dir != "" {
		mkdirResult, err := t.executor.Exec(ctx, nodeExecRequest{
			ContainerID: containerID,
			Args:        []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", shellQuote(dir))},
			Timeout:     10 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("create node file parent directory: %w", err)
		}
		if mkdirResult.ExitCode != 0 {
			return nil, fmt.Errorf("create node file parent directory failed: %s", strings.TrimSpace(combineOutput(mkdirResult.Stdout, mkdirResult.Stderr)))
		}
	}

	teeCommand := fmt.Sprintf("tee %s", shellQuote(path))
	if appendMode {
		teeCommand = fmt.Sprintf("tee -a %s", shellQuote(path))
	}
	writeResult, err := t.executor.Exec(ctx, nodeExecRequest{
		ContainerID: containerID,
		Args:        []string{"sh", "-c", teeCommand},
		Stdin:       &content,
		Timeout:     30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("write node file: %w", err)
	}
	if writeResult.TimedOut {
		return map[string]interface{}{
			"container_id": containerID,
			"path":         path,
			"timed_out":    true,
			"message":      fmt.Sprintf("write file timed out: %s", path),
		}, nil
	}
	if writeResult.ExitCode != 0 {
		return nil, fmt.Errorf("write node file failed: %s", strings.TrimSpace(combineOutput(writeResult.Stdout, writeResult.Stderr)))
	}

	action := "written"
	if appendMode {
		action = "appended"
	}
	lineCount := countContentLines(content)

	return map[string]interface{}{
		"container_id": containerID,
		"path":         path,
		"append":       appendMode,
		"action":       action,
		"line_count":   lineCount,
		"message":      fmt.Sprintf("file %s: %s (%d lines)", action, path, lineCount),
	}, nil
}
