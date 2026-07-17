package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

// testPermissionCore is a minimal in-package mock of account.PermissionProvider for tests.
type testPermissionCore struct {
	db *gorm.DB
}

func newTestPermissionCore(db *gorm.DB) PermissionCore {
	return &testPermissionCore{db: db}
}

func (t *testPermissionCore) Can(ctx context.Context, caller types.PermissionCaller, action types.Action, ref types.ResourceRef, input *types.MemberInput) (types.PermissionDecision, error) {
	return types.PermissionDecision{Allowed: true, Reason: "allowed"}, nil
}

func (t *testPermissionCore) Explain(ctx context.Context, caller types.PermissionCaller, action types.Action, ref types.ResourceRef, input *types.MemberInput) (types.PermissionExplainDecision, error) {
	return types.PermissionExplainDecision{PermissionDecision: types.PermissionDecision{Allowed: true, Reason: "allowed"}}, nil
}

func (t *testPermissionCore) BatchCan(ctx context.Context, caller types.PermissionCaller, actions []types.Action, refs []types.ResourceRef, input *types.MemberInput) ([]types.PermissionDecision, error) {
	results := make([]types.PermissionDecision, len(actions))
	for i := range results {
		results[i] = types.PermissionDecision{Allowed: true, Reason: "allowed"}
	}
	return results, nil
}

func (t *testPermissionCore) BatchCheckByPublicID(ctx context.Context, caller types.PermissionCaller, items []contract.BatchCheckPermissionItem) ([]contract.BatchCheckPermissionResult, error) {
	results := make([]contract.BatchCheckPermissionResult, len(items))
	for i, item := range items {
		results[i] = contract.BatchCheckPermissionResult{
			Action:       item.Action,
			ResourceType: item.ResourceType,
			PublicID:     item.PublicID,
			Allowed:      true,
			Reason:       "allowed",
		}
	}
	return results, nil
}

func (t *testPermissionCore) ResolveEffectiveRole(ctx context.Context, caller types.PermissionCaller, resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error) {
	return types.ResourceRoleOwner, nil, resource, nil
}

func newTestPermissionService(db *gorm.DB) *PermissionService {
	core := newTestPermissionCore(db)
	return NewPermissionService(db, core, func(d *gorm.DB) PermissionCore {
		return newTestPermissionCore(d)
	})
}
