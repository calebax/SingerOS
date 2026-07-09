package api

import (
	"context"

	"github.com/insmtx/Leros/backend/internal/api/handler"
	"github.com/insmtx/Leros/backend/internal/service"
	"github.com/insmtx/Leros/backend/types"
)

// NewPermissionBatchChecker 将 PermissionService 适配为 handler.PermissionBatchChecker。
func NewPermissionBatchChecker(svc *service.PermissionService) handler.PermissionBatchChecker {
	return &permissionBatchAdapter{svc: svc}
}

type permissionBatchAdapter struct {
	svc *service.PermissionService
}

func (a *permissionBatchAdapter) BatchCheckByPublicID(
	ctx context.Context,
	caller types.PermissionCaller,
	items []handler.PermissionBatchCheckItem,
) ([]handler.PermissionBatchCheckResult, error) {
	serviceItems := make([]service.BatchCheckItem, len(items))
	for i, item := range items {
		serviceItems[i] = service.BatchCheckItem{
			Action:       service.Action(item.Action),
			ResourceType: item.ResourceType,
			PublicID:     item.PublicID,
		}
	}

	results, err := a.svc.BatchCheckByPublicID(ctx, service.Caller(caller), serviceItems)
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
