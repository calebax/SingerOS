package main

import (
	"github.com/insmtx/Leros/backend/gateway/adapters/feishu"
	"github.com/insmtx/Leros/backend/gateway/adapters/github"
	"github.com/insmtx/Leros/backend/gateway/adapters/qqbot"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// newGitHubAdapter creates a GitHub webhook adapter from channel config.
func newGitHubAdapter(cfg types.ChannelConfig) *github.Adapter {
	return github.NewAdapter(cfg)
}

// newQQBotAdapter creates a QQ Bot adapter from channel config.
func newQQBotAdapter(cfg types.ChannelConfig) *qqbot.Adapter {
	return qqbot.NewAdapter(cfg)
}

// newFeishuAdapter creates a Feishu adapter from channel config.
func newFeishuAdapter(cfg types.ChannelConfig) *feishu.Adapter {
	return feishu.NewAdapter(cfg)
}
