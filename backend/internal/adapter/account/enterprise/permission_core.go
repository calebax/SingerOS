//go:build enterprise

package enterprise

import (
	"context"
	"github.com/insmtx/Leros/backend/pkg/accounterror"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

type permission struct{}

func NewPermission() *permission {
	return &permission{}
}

func (s *permission) Can(ctx context.Context, caller types.PermissionCaller, action types.Action, ref types.ResourceRef, input *types.MemberInput) (types.PermissionDecision, error) {
	return types.PermissionDecision{}, accounterror.ErrNotImplementedEdition
}

func (s *permission) Explain(ctx context.Context, caller types.PermissionCaller, action types.Action, ref types.ResourceRef, input *types.MemberInput) (types.PermissionExplainDecision, error) {
	return types.PermissionExplainDecision{}, accounterror.ErrNotImplementedEdition
}

func (s *permission) BatchCan(ctx context.Context, caller types.PermissionCaller, actions []types.Action, refs []types.ResourceRef, input *types.MemberInput) ([]types.PermissionDecision, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *permission) BatchCheckByPublicID(ctx context.Context, caller types.PermissionCaller, items []contract.BatchCheckPermissionItem) ([]contract.BatchCheckPermissionResult, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *permission) ResolveEffectiveRole(ctx context.Context, caller types.PermissionCaller, resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error) {
	return "", nil, nil, accounterror.ErrNotImplementedEdition
}
