package config

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestConfigParsesWorkspaceRootAndLogLevel(t *testing.T) {
	var cfg Config
	body := []byte("workspace_root: /tmp/leros\nlog:\n  level: error\nserver:\n  port: \"8080\"\n")

	if err := yaml.Unmarshal(body, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.WorkspaceRoot != "/tmp/leros" {
		t.Fatalf("workspace root = %q", cfg.WorkspaceRoot)
	}
	if cfg.Log.Level != "error" {
		t.Fatalf("log level = %q", cfg.Log.Level)
	}
}

func TestConfigParsesMCPConnectors(t *testing.T) {
	var cfg Config
	body := []byte(`mcp_connectors:
  - channel: netease-mail
    name: 邮箱
    status: active
    skill_code: connector-netease-mail
    bindings:
      skill_env:
        NETEASE_EMAIL_USER: email
    auth:
      type: form
      fields:
        - key: email
          label: 邮箱地址
          type: text
          required: true
  - channel: baidu-netdisk
    name: 百度网盘
    status: inactive
    transport: sse
    url: https://mcp-pan.baidu.com/sse
    auth:
      type: oauth
      handler: baidu-netdisk
      oauth:
        app_key: ${BAIDU_NETDISK_APP_KEY}
        secret_key: ${BAIDU_NETDISK_SECRET_KEY}
        redirect_uri: https://leros.example.com/callback
        scopes: [basic, netdisk]
`)

	if err := yaml.Unmarshal(body, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if len(cfg.MCPConnectors) != 2 {
		t.Fatalf("MCP connectors = %#v", cfg.MCPConnectors)
	}
	mail := cfg.MCPConnectors[0]
	if mail.SkillCode != "connector-netease-mail" || mail.Auth.Type != "form" ||
		len(mail.Auth.Fields) != 1 || mail.Bindings.SkillEnv["NETEASE_EMAIL_USER"] != "email" {
		t.Fatalf("mail connector = %#v", mail)
	}
	baidu := cfg.MCPConnectors[1]
	if baidu.Status != "inactive" || baidu.Auth.OAuth == nil ||
		baidu.Auth.OAuth.Scopes[1] != "netdisk" {
		t.Fatalf("baidu connector = %#v", baidu)
	}
}
