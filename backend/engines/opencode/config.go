package opencode

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/insmtx/Leros/backend/engines"
)

const (
	// providerID 是 OpenCode 配置中使用的 provider 标识符。
	providerID = "leros-provider"
	// providerNpm 使用 @ai-sdk/openai-compatible 通配大多数兼容 API。
	providerNpm = "@ai-sdk/openai-compatible"
)

// buildConfigContent 根据 ModelConfig 生成 OPENCODE_CONFIG_CONTENT JSON 字符串。
func buildConfigContent(modelCfg engines.ModelConfig) (string, error) {
	modelID := modelCfg.Model
	if modelID == "" {
		modelID = "default"
	}
	modelName := modelID
	if modelCfg.Provider != "" {
		modelName = modelCfg.Provider + "/" + modelID
	}

	cfg := configContent{
		Provider: map[string]providerConfig{
			providerID: {
				ID:  providerID,
				Npm: providerNpm,
				Options: providerOptions{
					APIKey:  modelCfg.APIKey,
					BaseURL: modelCfg.BaseURL,
				},
				Models: map[string]modelConfig{
					modelID: {
						ID:          modelID,
						Name:        modelName,
						ToolCall:    true,
						Attachment:  true,
						Reasoning:   false,
						Temperature: true,
						Limit: modelLimit{
							Context: 200000,
							Output:  16384,
						},
					},
				},
			},
		},
		Model: providerID + "/" + modelID,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config content: %w", err)
	}
	return string(data), nil
}

// buildServerEnv 构建 opcode serve 子进程所需的环境变量。
// 返回格式为 "KEY=VALUE" 的字符串切片，附加到 baseEnv 之后。
func buildServerEnv(password, configContent string, baseEnv []string) []string {
	env := make([]string, 0, len(baseEnv)+12)

	// 复制 base 环境变量
	env = append(env, baseEnv...)

	// 服务器认证
	env = append(env, "OPENCODE_SERVER_PASSWORD="+password)
	env = append(env, "OPENCODE_SERVER_USERNAME=opencode")

	// 注入完整配置（provider、model、API key、base URL）
	env = append(env, "OPENCODE_CONFIG_CONTENT="+configContent)

	// 隔离环境变量：确保子进程不读取宿主机的配置文件或插件
	env = append(env, "OPENCODE_DISABLE_PROJECT_CONFIG=1")
	env = append(env, "OPENCODE_PURE=1")
	env = append(env, "OPENCODE_DISABLE_AUTOUPDATE=1")
	env = append(env, "OPENCODE_DISABLE_AUTOCOMPACT=1")
	env = append(env, "OPENCODE_DISABLE_MODELS_FETCH=1")

	// 启用 v2 事件系统（session.next.* 事件流）
	env = append(env, "OPENCODE_EXPERIMENTAL_EVENT_SYSTEM=true")

	return env
}

// generatePassword 生成 32 位随机十六进制密码。
func generatePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random password: %w", err)
	}
	return hex.EncodeToString(b), nil
}
