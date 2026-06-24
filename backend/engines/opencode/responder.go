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

// WriteDecision 将审批决策写回 OpenCode 服务。
func (r *serverResponder) WriteDecision(requestID string, action string) error {
	decision := "deny"
	if action == engines.ApprovalActionApprove || action == engines.ApprovalActionAlways {
		decision = "approve"
	}

	if err := r.srv.SendPermissionDecision(context.Background(), r.sessionID, requestID, decision); err != nil {
		return fmt.Errorf("respond approval: %w", err)
	}
	return nil
}
