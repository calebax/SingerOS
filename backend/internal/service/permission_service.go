package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

// PermissionService 是统一资源权限判断服务。
// 所有业务入口通过 Can/Explain/BatchCan 判断当前主体对资源的操作是否被允许。
type PermissionService struct {
	db *gorm.DB
}

// NewPermissionService 创建权限服务实例。
func NewPermissionService(d *gorm.DB) *PermissionService {
	return &PermissionService{db: d}
}

// Caller 是 types.PermissionCaller 的别名，表示当前请求主体。
// Uin 与 AssistantID 互斥：普通用户登录时 Uin 非 0，助手身份时 AssistantID 非 0。
type Caller = types.PermissionCaller

// ResourceRef 是对 types.ResourceRef 的类型别名，便于在 service 包内直接使用。
type ResourceRef = types.ResourceRef

// Decision 是单次权限判断结果，对 types.PermissionDecision 的类型别名。
type Decision = types.PermissionDecision

// ExplainDecision 在 Decision 基础上增加继承来源信息，对 types.PermissionExplainDecision 的类型别名。
type ExplainDecision = types.PermissionExplainDecision

// MemberInput 是成员管理类动作的请求上下文，对 types.MemberInput 的类型别名。
type MemberInput = types.MemberInput

// MemberAuthContext 是项目成员管理动作派生的可信上下文。
// 其中所有字段必须由后端根据数据库计算，不允许前端传入。
type MemberAuthContext struct {
	TargetRole  types.ResourceRole
	NewRole     types.ResourceRole
	IsSelf      bool
	IsLastOwner bool
}

const (
	reasonAllowed             = "allowed"
	reasonNoBinding           = "no_binding"
	reasonOrgMismatch         = "org_mismatch"
	reasonResourceNotFound    = "resource_not_found"
	reasonPolicyDenied        = "policy_denied"
	reasonMemberContextDenied = "member_context_denied"
)

type refGroupKey struct {
	resourceType types.ResourceType
	bizID        uint
}

// Can 判断 caller 是否允许对指定资源执行 action。
// input 仅在成员管理类动作时需要传入；非成员管理动作可传 nil。
func (s *PermissionService) Can(ctx context.Context, caller Caller, action Action, ref ResourceRef, input *MemberInput) (Decision, error) {
	return s.evaluate(ctx, caller, action, ref, input, false)
}

// Explain 返回 caller 对指定资源的权限解释，包含最终角色和继承来源。
// input 仅在成员管理类动作时需要传入；非成员管理动作可传 nil。
func (s *PermissionService) Explain(ctx context.Context, caller Caller, action Action, ref ResourceRef, input *MemberInput) (ExplainDecision, error) {
	decision, err := s.evaluate(ctx, caller, action, ref, input, true)
	if err != nil {
		return ExplainDecision{}, err
	}

	explain := ExplainDecision{PermissionDecision: decision}
	if decision.Allowed {
		// 说明权限来自继承：当匹配到的资源 ID 与目标资源 ID 不一致时，即为继承。
		explain.Inherited = decision.MatchedResourceID != 0 && decision.MatchedResourceID != decision.ResourceID
	}
	return explain, nil
}

// BatchCan 批量判断 caller 对多个资源/动作是否允许。
// actions 与 refs 长度必须一致，按索引一一对应。
// 同一资源的多个 action 只加载 resource 并 resolve effective role 一次。
func (s *PermissionService) BatchCan(ctx context.Context, caller Caller, actions []Action, refs []ResourceRef, input *MemberInput) ([]Decision, error) {
	if len(actions) != len(refs) {
		return nil, errors.New("actions and refs must have the same length")
	}
	if len(actions) == 0 {
		return nil, nil
	}

	results := make([]Decision, len(actions))
	groups := make(map[refGroupKey][]int)
	for i := range actions {
		key := refGroupKey{resourceType: refs[i].Type, bizID: refs[i].BizID}
		groups[key] = append(groups[key], i)
	}

	for key, indices := range groups {
		ref := ResourceRef{Type: key.resourceType, BizID: key.bizID}
		groupActions := make([]Action, len(indices))
		for j, idx := range indices {
			groupActions[j] = actions[idx]
		}

		decisions, err := s.evaluateActions(ctx, caller, ref, groupActions, input)
		if err != nil {
			return nil, err
		}
		for j, idx := range indices {
			results[idx] = decisions[j]
		}
	}
	return results, nil
}

// evaluateActions 对同一资源批量评估多个 action。
func (s *PermissionService) evaluateActions(ctx context.Context, caller Caller, ref ResourceRef, actions []Action, input *MemberInput) ([]Decision, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	resource, err := s.loadResourceForEval(ctx, caller, ref)
	if err != nil {
		return nil, err
	}
	if resource == nil {
		denied := denyDecision(reasonResourceNotFound, 0)
		out := make([]Decision, len(actions))
		for i := range out {
			out[i] = denied
		}
		return out, nil
	}
	if resource.OrgID != caller.OrgID {
		denied := denyDecision(reasonOrgMismatch, resource.ID)
		out := make([]Decision, len(actions))
		for i := range out {
			out[i] = denied
		}
		return out, nil
	}

	return s.evaluateLoadedActions(ctx, caller, resource, actions, input)
}

// evaluate 是 Can 与 Explain 的公共实现。
func (s *PermissionService) evaluate(ctx context.Context, caller Caller, action Action, ref ResourceRef, input *MemberInput, explain bool) (Decision, error) {
	_ = explain
	if caller.OrgID == 0 {
		return denyDecision(reasonOrgMismatch, 0), nil
	}

	decisions, err := s.evaluateActions(ctx, caller, ref, []Action{action}, input)
	if err != nil {
		return Decision{}, err
	}
	return decisions[0], nil
}

func (s *PermissionService) loadResourceForEval(ctx context.Context, caller Caller, ref ResourceRef) (*types.Resource, error) {
	resource, err := db.GetResourceByBizID(ctx, s.db, caller.OrgID, ref.Type, ref.BizID)
	if err != nil {
		return nil, fmt.Errorf("load resource: %w", err)
	}
	return resource, nil
}

// evaluateLoadedActions 在 resource 已加载的前提下批量评估多个 action。
func (s *PermissionService) evaluateLoadedActions(ctx context.Context, caller Caller, resource *types.Resource, actions []Action, input *MemberInput) ([]Decision, error) {
	effectiveRole, matchedBinding, matchedResource, err := s.resolveEffectiveRole(ctx, caller, resource)
	if err != nil {
		return nil, fmt.Errorf("resolve effective role: %w", err)
	}

	decisions := make([]Decision, len(actions))
	for i, action := range actions {
		if effectiveRole == "" {
			decisions[i] = denyDecision(reasonNoBinding, resource.ID)
			continue
		}
		if !SystemPolicy.Allows(resource.Type, effectiveRole, action) {
			decisions[i] = denyDecision(reasonPolicyDenied, resource.ID)
			continue
		}
		if IsMemberManagementAction(action) {
			authCtx, buildErr := s.buildMemberAuthContext(ctx, caller, resource, effectiveRole, input)
			if buildErr != nil {
				return nil, fmt.Errorf("build member auth context: %w", buildErr)
			}
			if !s.memberActionAllowed(action, effectiveRole, authCtx) {
				decisions[i] = denyDecision(reasonMemberContextDenied, resource.ID)
				continue
			}
		}
		decision := Decision{
			Allowed:    true,
			Reason:     reasonAllowed,
			Role:       effectiveRole,
			ResourceID: resource.ID,
		}
		if matchedBinding != nil {
			decision.MatchedBindingID = matchedBinding.ID
		}
		if matchedResource != nil {
			decision.MatchedResourceID = matchedResource.ID
		}
		decisions[i] = decision
	}
	return decisions, nil
}

// resolveEffectiveRole 计算 caller 在目标资源上的最终角色。
// 逻辑：
// 1. 查找当前资源上的直接 binding；
// 2. 沿 parent_resource_id 向上查找祖先 binding；
// 3. 汇总所有角色，用 MaxRole 取强度最高者。
func (s *PermissionService) resolveEffectiveRole(ctx context.Context, caller Caller, resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error) {
	var roles []types.ResourceRole
	var matchedBinding *types.ResourceBinding
	var matchedResource *types.Resource

	// 直接 binding
	direct, err := s.findDirectBinding(ctx, caller, resource.ID)
	if err != nil {
		return "", nil, nil, err
	}
	if direct != nil {
		roles = append(roles, direct.Role)
		matchedBinding = direct
		matchedResource = resource
	}

	// 沿资源树向上查找祖先 binding
	current := resource
	for current.ParentResourceID != nil && *current.ParentResourceID != 0 {
		parent, err := db.GetResourceByID(ctx, s.db, *current.ParentResourceID)
		if err != nil {
			return "", nil, nil, err
		}
		if parent == nil {
			break
		}
		binding, err := s.findDirectBinding(ctx, caller, parent.ID)
		if err != nil {
			return "", nil, nil, err
		}
		if binding != nil {
			roles = append(roles, binding.Role)
			if matchedBinding == nil {
				matchedBinding = binding
				matchedResource = parent
			}
		}
		current = parent
	}

	if len(roles) == 0 {
		return "", nil, nil, nil
	}
	return MaxRole(roles), matchedBinding, matchedResource, nil
}

// findDirectBinding 查找 caller 在指定资源上的直接 binding。
func (s *PermissionService) findDirectBinding(ctx context.Context, caller Caller, resourceID uint) (*types.ResourceBinding, error) {
	if caller.Uin != 0 {
		return db.GetResourceBindingByUin(ctx, s.db, resourceID, caller.Uin)
	}
	if caller.AssistantID != 0 {
		return db.GetResourceBindingByAssistantID(ctx, s.db, resourceID, caller.AssistantID)
	}
	return nil, nil
}

// buildMemberAuthContext 根据请求输入和数据库状态派生成员管理上下文。
func (s *PermissionService) buildMemberAuthContext(ctx context.Context, caller Caller, resource *types.Resource, operatorRole types.ResourceRole, input *MemberInput) (MemberAuthContext, error) {
	ctxOut := MemberAuthContext{}
	if input == nil {
		return ctxOut, nil
	}

	// 判断是否操作自己
	if input.TargetAssistantID == nil && input.TargetUin == caller.Uin {
		ctxOut.IsSelf = true
	}
	if input.TargetAssistantID != nil && caller.AssistantID != 0 && *input.TargetAssistantID == caller.AssistantID {
		ctxOut.IsSelf = true
	}

	ctxOut.NewRole = input.RequestedRole

	// 查询目标当前角色
	var targetBinding *types.ResourceBinding
	if input.TargetAssistantID == nil && input.TargetUin != 0 {
		b, err := db.GetResourceBindingByUin(ctx, s.db, resource.ID, input.TargetUin)
		if err != nil {
			return ctxOut, err
		}
		targetBinding = b
	} else if input.TargetAssistantID != nil && *input.TargetAssistantID != 0 {
		b, err := db.GetResourceBindingByAssistantID(ctx, s.db, resource.ID, *input.TargetAssistantID)
		if err != nil {
			return ctxOut, err
		}
		targetBinding = b
	}

	if targetBinding != nil {
		ctxOut.TargetRole = targetBinding.Role
	}

	// 判断目标是否为最后一个 owner
	if ctxOut.TargetRole == types.ResourceRoleOwner {
		ownerCount, err := db.CountResourceBindingsByRole(ctx, s.db, resource.ID, types.ResourceRoleOwner)
		if err != nil {
			return ctxOut, err
		}
		ctxOut.IsLastOwner = ownerCount <= 1
	}

	return ctxOut, nil
}

// memberActionAllowed 根据设计文档中的成员管理规则判断最终是否允许。
func (s *PermissionService) memberActionAllowed(action Action, operatorRole types.ResourceRole, ctx MemberAuthContext) bool {
	if action == ActionProjectMemberLeave {
		if !ctx.IsSelf {
			return false
		}
		if ctx.IsLastOwner {
			return false
		}
		return true
	}

	switch operatorRole {
	case types.ResourceRoleOwner:
		// owner 可以创建/更新/删除任意角色，但不能让项目失去最后一个 owner。
		if ctx.IsLastOwner && (action == ActionProjectMemberDelete || (action == ActionProjectMemberUpdate && ctx.TargetRole == types.ResourceRoleOwner && ctx.NewRole != types.ResourceRoleOwner)) {
			return false
		}
		return true
	case types.ResourceRoleAdmin:
		// admin 不能操作 owner，不能把任何人提升为 owner。
		if ctx.TargetRole == types.ResourceRoleOwner {
			return false
		}
		if ctx.NewRole == types.ResourceRoleOwner {
			return false
		}
		return action == ActionProjectMemberCreate || action == ActionProjectMemberUpdate || action == ActionProjectMemberDelete
	case types.ResourceRoleMember:
		// member 只能查看成员列表。
		return action == ActionProjectMemberList
	}
	return false
}

// denyDecision 构造拒绝 Decision。
func denyDecision(reason string, resourceID uint) Decision {
	return Decision{
		Allowed:    false,
		Reason:     reason,
		ResourceID: resourceID,
	}
}
