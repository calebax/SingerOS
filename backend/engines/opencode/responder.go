package opencode

import (
	"context"
	"fmt"

	"github.com/insmtx/Leros/backend/engines"
)

// serverResponder 通过 OpenCode HTTP API 响应审批请求。
type serverResponder struct {
	srv       *OpenCodeServer
	sessionID string
}

// WriteDecision 将审批决策转换为 OpenCode HTTP API 权限响应。
// Leros 审批动作 → OpenCode PermissionV1.Reply:
//
//	"approve" → "once"    仅本次允许
//	"always"  → "always"  始终允许
//	"deny"    → "reject"  拒绝
func (r *serverResponder) WriteDecision(requestID string, action string) error {
	var reply string
	switch action {
	case engines.ApprovalActionApprove:
		reply = "once"
	case engines.ApprovalActionAlways:
		reply = "always"
	default:
		reply = "reject"
	}

	if err := r.srv.SendPermissionDecision(context.Background(), r.sessionID, requestID, reply); err != nil {
		return fmt.Errorf("respond approval: %w", err)
	}
	return nil
}
