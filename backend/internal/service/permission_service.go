package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

// PermissionCore is the minimal interface that PermissionService requires
// to delegate permission evaluation. Both oss and enterprise adapters
// satisfy this contract.
type PermissionCore interface {
	Can(ctx context.Context, caller types.PermissionCaller, action types.Action,
		ref types.ResourceRef, input *types.MemberInput) (types.PermissionDecision, error)
	Explain(ctx context.Context, caller types.PermissionCaller, action types.Action,
		ref types.ResourceRef, input *types.MemberInput) (types.PermissionExplainDecision, error)
	BatchCan(ctx context.Context, caller types.PermissionCaller, actions []types.Action,
		refs []types.ResourceRef, input *types.MemberInput) ([]types.PermissionDecision, error)
	BatchCheckByPublicID(ctx context.Context, caller types.PermissionCaller,
		items []contract.BatchCheckPermissionItem) ([]contract.BatchCheckPermissionResult, error)
	ResolveEffectiveRole(ctx context.Context, caller types.PermissionCaller,
		resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error)
}

const (
	reasonPolicyDenied = "policy_denied"
)

// BatchCheckItem describes a single permission check request by public ID.
type BatchCheckItem struct {
	Action       Action
	ResourceType types.ResourceType
	PublicID     string
}

// BatchCheckResult is the result of a single permission check.
type BatchCheckResult struct {
	Action       Action
	ResourceType types.ResourceType
	PublicID     string
	Allowed      bool
	Reason       string
	Role         types.ResourceRole
	Inherited    bool
}

// PermissionService 是统一资源权限判断服务。
// 包含 Guard/Require 便捷方法（本包内），核心权限评估委托给 PermissionCore 接口实现。
type PermissionService struct {
	db      *gorm.DB
	core    PermissionCore
	newCore func(db *gorm.DB) PermissionCore
}

// NewPermissionService 创建权限服务实例。
func NewPermissionService(d *gorm.DB, core PermissionCore, newCore func(db *gorm.DB) PermissionCore) *PermissionService {
	return &PermissionService{db: d, core: core, newCore: newCore}
}

// Caller 是 types.PermissionCaller 的别名，表示当前请求主体。
type Caller = types.PermissionCaller

// ResourceRef 是对 types.ResourceRef 的类型别名。
type ResourceRef = types.ResourceRef

// Decision 是单次权限判断结果，对 types.PermissionDecision 的类型别名。
type Decision = types.PermissionDecision

// ExplainDecision 在 Decision 基础上增加继承来源信息，对 types.PermissionExplainDecision 的类型别名。
type ExplainDecision = types.PermissionExplainDecision

// MemberInput 是成员管理类动作的请求上下文，对 types.MemberInput 的类型别名。
type MemberInput = types.MemberInput

// MemberAuthContext 是项目成员管理动作派生的可信上下文。
type MemberAuthContext struct {
	TargetRole  types.ResourceRole
	NewRole     types.ResourceRole
	IsSelf      bool
	IsLastOwner bool
}

// Can 委托给 core 接口实现。
func (s *PermissionService) Can(ctx context.Context, caller Caller, action Action, ref ResourceRef, input *MemberInput) (Decision, error) {
	return s.core.Can(ctx, caller, action, ref, input)
}

// Explain 委托给 core 接口实现。
func (s *PermissionService) Explain(ctx context.Context, caller Caller, action Action, ref ResourceRef, input *MemberInput) (ExplainDecision, error) {
	return s.core.Explain(ctx, caller, action, ref, input)
}

// BatchCan 委托给 core 接口实现。
func (s *PermissionService) BatchCan(ctx context.Context, caller Caller, actions []Action, refs []ResourceRef, input *MemberInput) ([]Decision, error) {
	return s.core.BatchCan(ctx, caller, actions, refs, input)
}

// BatchCheckByPublicID 委托给 core 接口实现。
func (s *PermissionService) BatchCheckByPublicID(ctx context.Context, caller Caller, items []BatchCheckItem) ([]BatchCheckResult, error) {
	coreItems := make([]contract.BatchCheckPermissionItem, len(items))
	for i, item := range items {
		coreItems[i] = contract.BatchCheckPermissionItem{
			Action:       item.Action,
			ResourceType: item.ResourceType,
			PublicID:     item.PublicID,
		}
	}
	results, err := s.core.BatchCheckByPublicID(ctx, caller, coreItems)
	if err != nil {
		return nil, err
	}
	out := make([]BatchCheckResult, len(results))
	for i, r := range results {
		out[i] = BatchCheckResult{
			Action:       r.Action,
			ResourceType: r.ResourceType,
			PublicID:     r.PublicID,
			Allowed:      r.Allowed,
			Reason:       r.Reason,
			Role:         r.Role,
			Inherited:    r.Inherited,
		}
	}
	return out, nil
}

// ResolveEffectiveRole delegates to core.ResolveEffectiveRole.
func (s *PermissionService) ResolveEffectiveRole(ctx context.Context, caller Caller, resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error) {
	return s.core.ResolveEffectiveRole(ctx, caller, resource)
}
