//go:build !enterprise

package oss

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/service"
	"github.com/insmtx/Leros/backend/types"
)

const (
	reasonAllowed             = "allowed"
	reasonNoBinding           = "no_binding"
	reasonOrgMismatch         = "org_mismatch"
	reasonResourceNotFound    = "resource_not_found"
	reasonPolicyDenied        = "policy_denied"
	reasonMemberContextDenied = "member_context_denied"
)

type memberAuthContext struct {
	TargetRole  types.ResourceRole
	NewRole     types.ResourceRole
	IsSelf      bool
	IsLastOwner bool
}

type refGroupKey struct {
	resourceType types.ResourceType
	bizID        uint
}

type permission struct {
	db *gorm.DB
}

func NewPermission(db *gorm.DB) *permission {
	return &permission{db: db}
}

func (s *permission) Can(ctx context.Context, caller types.PermissionCaller, action types.Action, ref types.ResourceRef, input *types.MemberInput) (types.PermissionDecision, error) {
	return s.evaluate(ctx, caller, action, ref, input, false)
}

func (s *permission) Explain(ctx context.Context, caller types.PermissionCaller, action types.Action, ref types.ResourceRef, input *types.MemberInput) (types.PermissionExplainDecision, error) {
	decision, err := s.evaluate(ctx, caller, action, ref, input, true)
	if err != nil {
		return types.PermissionExplainDecision{}, err
	}

	explain := types.PermissionExplainDecision{PermissionDecision: decision}
	if decision.Allowed {
		explain.Inherited = decision.MatchedResourceID != 0 && decision.MatchedResourceID != decision.ResourceID
	}
	return explain, nil
}

func (s *permission) BatchCan(ctx context.Context, caller types.PermissionCaller, actions []types.Action, refs []types.ResourceRef, input *types.MemberInput) ([]types.PermissionDecision, error) {
	if len(actions) != len(refs) {
		return nil, errors.New("actions and refs must have the same length")
	}
	if len(actions) == 0 {
		return nil, nil
	}

	results := make([]types.PermissionDecision, len(actions))
	groups := make(map[refGroupKey][]int)
	for i := range actions {
		key := refGroupKey{resourceType: refs[i].Type, bizID: refs[i].BizID}
		groups[key] = append(groups[key], i)
	}

	for key, indices := range groups {
		ref := types.ResourceRef{Type: key.resourceType, BizID: key.bizID}
		groupActions := make([]types.Action, len(indices))
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

func (s *permission) BatchCheckByPublicID(ctx context.Context, caller types.PermissionCaller, items []contract.BatchCheckPermissionItem) ([]contract.BatchCheckPermissionResult, error) {
	if len(items) == 0 {
		return nil, nil
	}

	results := make([]contract.BatchCheckPermissionResult, len(items))
	actions := make([]types.Action, len(items))
	refs := make([]types.ResourceRef, len(items))
	resolved := make([]bool, len(items))

	for i, item := range items {
		results[i] = contract.BatchCheckPermissionResult{
			Action:       item.Action,
			ResourceType: item.ResourceType,
			PublicID:     item.PublicID,
		}
		ref, err := s.resolveRefByPublicID(ctx, caller, item.ResourceType, item.PublicID)
		if err != nil {
			results[i].Allowed = false
			results[i].Reason = reasonResourceNotFound
			continue
		}
		actions[i] = item.Action
		refs[i] = ref
		resolved[i] = true
	}

	var checkActions []types.Action
	var checkRefs []types.ResourceRef
	indexMap := make([]int, 0, len(items))
	for i := range items {
		if !resolved[i] {
			continue
		}
		if actions[i] == types.ActionProjectMemberLeave {
			selfInput := &types.MemberInput{TargetUin: caller.Uin}
			decision, err := s.Can(ctx, caller, actions[i], refs[i], selfInput)
			if err != nil {
				return nil, err
			}
			results[i].Allowed = decision.Allowed
			results[i].Reason = decision.Reason
			results[i].Role = decision.Role
			if decision.Allowed {
				results[i].Inherited = decision.MatchedResourceID != 0 &&
					decision.MatchedResourceID != decision.ResourceID
			}
			continue
		}
		checkActions = append(checkActions, actions[i])
		checkRefs = append(checkRefs, refs[i])
		indexMap = append(indexMap, i)
	}

	if len(checkActions) == 0 {
		return results, nil
	}

	decisions, err := s.BatchCan(ctx, caller, checkActions, checkRefs, nil)
	if err != nil {
		return nil, err
	}

	for j, idx := range indexMap {
		decision := decisions[j]
		results[idx].Allowed = decision.Allowed
		results[idx].Reason = decision.Reason
		results[idx].Role = decision.Role
		if decision.Allowed {
			results[idx].Inherited = decision.MatchedResourceID != 0 &&
				decision.MatchedResourceID != decision.ResourceID
		}
	}
	return results, nil
}

func (s *permission) evaluateActions(ctx context.Context, caller types.PermissionCaller, ref types.ResourceRef, actions []types.Action, input *types.MemberInput) ([]types.PermissionDecision, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	resource, err := s.loadResourceForEval(ctx, caller, ref)
	if err != nil {
		return nil, err
	}
	if resource == nil {
		denied := denyDecision(reasonResourceNotFound, 0)
		out := make([]types.PermissionDecision, len(actions))
		for i := range out {
			out[i] = denied
		}
		return out, nil
	}
	if resource.OrgID != caller.OrgID {
		denied := denyDecision(reasonOrgMismatch, resource.ID)
		out := make([]types.PermissionDecision, len(actions))
		for i := range out {
			out[i] = denied
		}
		return out, nil
	}

	return s.evaluateLoadedActions(ctx, caller, resource, actions, input)
}

func (s *permission) evaluate(ctx context.Context, caller types.PermissionCaller, action types.Action, ref types.ResourceRef, input *types.MemberInput, explain bool) (types.PermissionDecision, error) {
	_ = explain
	if caller.OrgID == 0 {
		return denyDecision(reasonOrgMismatch, 0), nil
	}

	decisions, err := s.evaluateActions(ctx, caller, ref, []types.Action{action}, input)
	if err != nil {
		return types.PermissionDecision{}, err
	}
	return decisions[0], nil
}

func (s *permission) loadResourceForEval(ctx context.Context, caller types.PermissionCaller, ref types.ResourceRef) (*types.Resource, error) {
	resource, err := db.GetResourceByBizID(ctx, s.db, caller.OrgID, ref.Type, ref.BizID)
	if err != nil {
		return nil, fmt.Errorf("load resource: %w", err)
	}
	return resource, nil
}

func (s *permission) evaluateLoadedActions(ctx context.Context, caller types.PermissionCaller, resource *types.Resource, actions []types.Action, input *types.MemberInput) ([]types.PermissionDecision, error) {
	effectiveRole, matchedBinding, matchedResource, err := s.resolveEffectiveRole(ctx, caller, resource)
	if err != nil {
		return nil, fmt.Errorf("resolve effective role: %w", err)
	}

	decisions := make([]types.PermissionDecision, len(actions))
	for i, action := range actions {
		if effectiveRole == "" {
			decisions[i] = denyDecision(reasonNoBinding, resource.ID)
			continue
		}
		if !service.SystemPolicy.Allows(resource.Type, effectiveRole, action) {
			decisions[i] = denyDecision(reasonPolicyDenied, resource.ID)
			continue
		}
		if service.IsMemberManagementAction(action) {
			authCtx, buildErr := s.buildMemberAuthContext(ctx, caller, resource, effectiveRole, input)
			if buildErr != nil {
				return nil, fmt.Errorf("build member auth context: %w", buildErr)
			}
			if !s.memberActionAllowed(action, effectiveRole, authCtx) {
				decisions[i] = denyDecision(reasonMemberContextDenied, resource.ID)
				continue
			}
		}
		decision := types.PermissionDecision{
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

func (s *permission) ResolveEffectiveRole(ctx context.Context, caller types.PermissionCaller, resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error) {
	return s.resolveEffectiveRole(ctx, caller, resource)
}

func (s *permission) resolveEffectiveRole(ctx context.Context, caller types.PermissionCaller, resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error) {
	var roles []types.ResourceRole
	var matchedBinding *types.ResourceBinding
	var matchedResource *types.Resource

	direct, err := s.findDirectBinding(ctx, caller, resource.ID)
	if err != nil {
		return "", nil, nil, err
	}
	if direct != nil {
		roles = append(roles, direct.Role)
		matchedBinding = direct
		matchedResource = resource
	}

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
	return service.MaxRole(roles), matchedBinding, matchedResource, nil
}

func (s *permission) findDirectBinding(ctx context.Context, caller types.PermissionCaller, resourceID uint) (*types.ResourceBinding, error) {
	if caller.Uin != 0 {
		return db.GetResourceBindingByUin(ctx, s.db, resourceID, caller.Uin)
	}
	if caller.AssistantID != 0 {
		return db.GetResourceBindingByAssistantID(ctx, s.db, resourceID, caller.AssistantID)
	}
	return nil, nil
}

func (s *permission) buildMemberAuthContext(ctx context.Context, caller types.PermissionCaller, resource *types.Resource, operatorRole types.ResourceRole, input *types.MemberInput) (memberAuthContext, error) {
	ctxOut := memberAuthContext{}
	if input == nil {
		return ctxOut, nil
	}

	if input.TargetAssistantID == nil && input.TargetUin == caller.Uin {
		ctxOut.IsSelf = true
	}
	if input.TargetAssistantID != nil && caller.AssistantID != 0 && *input.TargetAssistantID == caller.AssistantID {
		ctxOut.IsSelf = true
	}

	ctxOut.NewRole = input.RequestedRole

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

	if ctxOut.TargetRole == types.ResourceRoleOwner {
		ownerCount, err := db.CountResourceBindingsByRole(ctx, s.db, resource.ID, types.ResourceRoleOwner)
		if err != nil {
			return ctxOut, err
		}
		ctxOut.IsLastOwner = ownerCount <= 1
	}

	return ctxOut, nil
}

func (s *permission) memberActionAllowed(action types.Action, operatorRole types.ResourceRole, ctx memberAuthContext) bool {
	if action == types.ActionProjectMemberLeave {
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
		if ctx.IsLastOwner && (action == types.ActionProjectMemberDelete || (action == types.ActionProjectMemberUpdate && ctx.TargetRole == types.ResourceRoleOwner && ctx.NewRole != types.ResourceRoleOwner)) {
			return false
		}
		return true
	case types.ResourceRoleAdmin:
		if ctx.TargetRole == types.ResourceRoleOwner {
			return false
		}
		if ctx.NewRole == types.ResourceRoleOwner {
			return false
		}
		return action == types.ActionProjectMemberCreate || action == types.ActionProjectMemberUpdate || action == types.ActionProjectMemberDelete
	case types.ResourceRoleMember:
		return action == types.ActionProjectMemberList
	}
	return false
}

func (s *permission) resolveRefByPublicID(ctx context.Context, caller types.PermissionCaller, resourceType types.ResourceType, publicID string) (types.ResourceRef, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return types.ResourceRef{}, fmt.Errorf("public_id is required")
	}

	switch resourceType {
	case types.ResourceTypeProject:
		project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, publicID)
		if err != nil {
			return types.ResourceRef{}, err
		}
		if project == nil {
			return types.ResourceRef{}, fmt.Errorf("project not found")
		}
		return types.ResourceRef{Type: types.ResourceTypeProject, BizID: project.ID}, nil
	case types.ResourceTypeTask:
		task, err := db.GetTaskByPublicID(ctx, s.db, caller.OrgID, publicID)
		if err != nil {
			return types.ResourceRef{}, err
		}
		if task == nil {
			return types.ResourceRef{}, fmt.Errorf("task not found")
		}
		return types.ResourceRef{Type: types.ResourceTypeTask, BizID: task.ID}, nil
	case types.ResourceTypeFile:
		file, err := db.GetProjectFileByFilePublicID(ctx, s.db, caller.OrgID, publicID)
		if err != nil {
			return types.ResourceRef{}, err
		}
		if file == nil {
			return types.ResourceRef{}, fmt.Errorf("file not found")
		}
		return types.ResourceRef{Type: types.ResourceTypeFile, BizID: file.ID}, nil
	default:
		return types.ResourceRef{}, fmt.Errorf("unsupported resource type %q", resourceType)
	}
}

func denyDecision(reason string, resourceID uint) types.PermissionDecision {
	return types.PermissionDecision{
		Allowed:    false,
		Reason:     reason,
		ResourceID: resourceID,
	}
}
