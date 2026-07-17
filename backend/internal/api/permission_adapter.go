package api

import (
	"context"

	adapteraccount "github.com/insmtx/Leros/backend/internal/adapter/account"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/api/handler"
	"github.com/insmtx/Leros/backend/types"
)

// NewPermissionBatchChecker 将 account.PermissionProvider 适配为 handler.PermissionBatchChecker。
func NewPermissionBatchChecker(svc adapteraccount.PermissionProvider) handler.PermissionBatchChecker {
	return &permissionBatchAdapter{svc: svc}
}

type permissionBatchAdapter struct {
	svc adapteraccount.PermissionProvider
}

func (a *permissionBatchAdapter) BatchCheckByPublicID(
	ctx context.Context,
	caller types.PermissionCaller,
	items []handler.PermissionBatchCheckItem,
) ([]handler.PermissionBatchCheckResult, error) {
	coreItems := make([]contract.BatchCheckPermissionItem, len(items))
	for i, item := range items {
		coreItems[i] = contract.BatchCheckPermissionItem{
			Action:       types.Action(item.Action),
			ResourceType: item.ResourceType,
			PublicID:     item.PublicID,
		}
	}

	results, err := a.svc.BatchCheckByPublicID(ctx, caller, coreItems)
	if err != nil {
		return nil, err
	}

	resp := make([]handler.PermissionBatchCheckResult, len(results))
	for i, r := range results {
		resp[i] = handler.PermissionBatchCheckResult{
			Action:       string(r.Action),
			ResourceType: r.ResourceType,
			PublicID:     r.PublicID,
			Allowed:      r.Allowed,
			Reason:       r.Reason,
			Role:         string(r.Role),
			Inherited:    r.Inherited,
		}
	}
	return resp, nil
}
