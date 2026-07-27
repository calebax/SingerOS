package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ArtifactDefinition identifies an immutable bundle through its FileUpload record.
type ArtifactDefinition struct {
	FileUploadID string `json:"file_upload_id"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"size_bytes"`
	ContentType  string `json:"content_type"`
}

// SkillSourceDefinition identifies a non-bundle Skill source such as GitHub.
type SkillSourceDefinition struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type skillDefinition struct {
	Schema   string                 `json:"schema"`
	Artifact *ArtifactDefinition    `json:"artifact"`
	Source   *SkillSourceDefinition `json:"source"`
}
type mcpDefinition struct {
	Schema        string            `json:"schema"`
	Transport     string            `json:"transport"`
	Name          string            `json:"name"`
	URL           string            `json:"url"`
	Command       string            `json:"command"`
	Args          []string          `json:"args"`
	SecretRefs    map[string]string `json:"secret_refs"`
	EnvSecretRefs map[string]string `json:"env_secret_refs"`
}
type workflowDefinition struct {
	Schema     string          `json:"schema"`
	Definition json.RawMessage `json:"definition"`
}

// ValidatePluginDefinition validates the JSON stored and transported for one plugin kind.
func ValidatePluginDefinition(kind string, raw json.RawMessage) error {
	if !json.Valid(raw) {
		return fmt.Errorf("definition must be valid JSON")
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "skill":
		var value skillDefinition
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		if value.Schema != "skill/v1" {
			return fmt.Errorf("unsupported skill definition schema %q", value.Schema)
		}
		if value.Artifact != nil && strings.TrimSpace(value.Artifact.FileUploadID) != "" && strings.TrimSpace(value.Artifact.SHA256) != "" {
			return nil
		}
		if value.Source != nil && strings.EqualFold(value.Source.Type, "github") && strings.TrimSpace(value.Source.URL) != "" {
			return nil
		}
		return fmt.Errorf("skill definition requires artifact file_upload_id and sha256, or a github source")
	case "mcp":
		var value mcpDefinition
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		if value.Schema != "mcp/v1" {
			return fmt.Errorf("unsupported mcp definition schema %q", value.Schema)
		}
		switch value.Transport {
		case "http":
			if strings.TrimSpace(value.URL) == "" {
				return fmt.Errorf("mcp http definition requires url")
			}
		case "stdio":
			if strings.TrimSpace(value.Command) == "" {
				return fmt.Errorf("mcp stdio definition requires command")
			}
		default:
			return fmt.Errorf("unsupported mcp transport %q", value.Transport)
		}
		return nil
	case "workflow":
		var value workflowDefinition
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		if value.Schema != "workflow/v1" {
			return fmt.Errorf("unsupported workflow definition schema %q", value.Schema)
		}
		if !json.Valid(value.Definition) || len(value.Definition) == 0 {
			return fmt.Errorf("workflow definition is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported plugin kind %q", kind)
	}
}

// ArtifactFromDefinition obtains the bundle metadata used by worker download code.
func ArtifactFromDefinition(kind string, raw json.RawMessage) (*ArtifactDefinition, error) {
	if strings.ToLower(kind) != "skill" {
		return nil, nil
	}
	if err := ValidatePluginDefinition(kind, raw); err != nil {
		return nil, err
	}
	var value skillDefinition
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value.Artifact, nil
}

func isBundleDefinition(kind string) bool { return strings.EqualFold(strings.TrimSpace(kind), "skill") }
