package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

// BatchCheckItem 描述单次基于 public_id 的权限检查请求。
type BatchCheckItem struct {
	Action       Action
	ResourceType types.ResourceType
	PublicID     string
}

// BatchCheckResult 是单次权限检查的返回结果，public_id 与请求项对应。
type BatchCheckResult struct {
	Action       Action
	ResourceType types.ResourceType
	PublicID     string
	Allowed      bool
	Reason       string
	Role         types.ResourceRole
	Inherited    bool
}

// BatchCheckByPublicID 批量判断 caller 对多个资源/动作是否允许。
// items 按索引与返回结果一一对应；解析 public_id 失败时该条 allowed=false。
func (s *PermissionService) BatchCheckByPublicID(ctx context.Context, caller Caller, items []BatchCheckItem) ([]BatchCheckResult, error) {
	if len(items) == 0 {
		return nil, nil
	}

	results := make([]BatchCheckResult, len(items))
	actions := make([]Action, len(items))
	refs := make([]ResourceRef, len(items))
	resolved := make([]bool, len(items))

	for i, item := range items {
		results[i] = BatchCheckResult{
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

	var checkActions []Action
	var checkRefs []ResourceRef
	indexMap := make([]int, 0, len(items))
	for i := range items {
		if !resolved[i] {
			continue
		}
		if actions[i] == ActionProjectMemberLeave {
			selfInput := &MemberInput{TargetUin: caller.Uin}
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

func (s *PermissionService) resolveRefByPublicID(
	ctx context.Context,
	caller Caller,
	resourceType types.ResourceType,
	publicID string,
) (ResourceRef, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return ResourceRef{}, fmt.Errorf("public_id is required")
	}

	switch resourceType {
	case types.ResourceTypeProject:
		project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, publicID)
		if err != nil {
			return ResourceRef{}, err
		}
		if project == nil {
			return ResourceRef{}, fmt.Errorf("project not found")
		}
		return ResourceRef{Type: types.ResourceTypeProject, BizID: project.ID}, nil
	case types.ResourceTypeTask:
		task, err := db.GetTaskByPublicID(ctx, s.db, caller.OrgID, publicID)
		if err != nil {
			return ResourceRef{}, err
		}
		if task == nil {
			return ResourceRef{}, fmt.Errorf("task not found")
		}
		return ResourceRef{Type: types.ResourceTypeTask, BizID: task.ID}, nil
	case types.ResourceTypeFile:
		file, err := db.GetProjectFileByFilePublicID(ctx, s.db, caller.OrgID, publicID)
		if err != nil {
			return ResourceRef{}, err
		}
		if file == nil {
			return ResourceRef{}, fmt.Errorf("file not found")
		}
		return ResourceRef{Type: types.ResourceTypeFile, BizID: file.ID}, nil
	default:
		return ResourceRef{}, fmt.Errorf("unsupported resource type %q", resourceType)
	}
}
