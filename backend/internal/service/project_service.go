package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ygpkg/storage-go"

	"code.gitea.io/sdk/gitea"

	"github.com/insmtx/Leros/backend/config"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/infra/filestore"
	"github.com/insmtx/Leros/backend/internal/infra/git"
	localmemory "github.com/insmtx/Leros/backend/internal/memory/local"
	"github.com/insmtx/Leros/backend/internal/workspace"
	"github.com/insmtx/Leros/backend/types"
	"github.com/ygpkg/yg-go/encryptor/snowflake"
	"github.com/ygpkg/yg-go/logs"
)

const (
	createdAtMaxConcurrent = 8
	createdAtMaxPages      = 100
)

type projectActivityCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uint      `json:"id"`
}

type projectService struct {
	db          *gorm.DB
	inferrer    AssistantInferrer
	giteaClient *gitea.Client
	giteaCfg    *config.GiteaConfig
	env         string
}

// fileTreeEntry 文件树 walk 阶段收集的扁平条目
type fileTreeEntry struct {
	absPath string
	isDir   bool
	size    int64
	modTime int64
}

// NewProjectService 创建项目服务实例
func NewProjectService(db *gorm.DB, giteaClient *gitea.Client, giteaCfg *config.GiteaConfig, env string) contract.ProjectService {
	return &projectService{
		db:          db,
		giteaClient: giteaClient,
		giteaCfg:    giteaCfg,
		env:         env,
	}
}

func NewProjectServiceWithInferrer(db *gorm.DB, inferrer AssistantInferrer, giteaClient *gitea.Client, giteaCfg *config.GiteaConfig, env string) contract.ProjectService {
	return &projectService{
		db:          db,
		inferrer:    inferrer,
		giteaClient: giteaClient,
		giteaCfg:    giteaCfg,
		env:         env,
	}
}

func (s *projectService) CreateProject(ctx context.Context, req *contract.CreateProjectRequest) (*contract.Project, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("name is required")
	}

	publicID := generateProjectPublicID()

	project := &types.Project{
		OrgID:       caller.OrgID,
		PublicID:    publicID,
		OwnerID:     caller.Uin,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Objective:   strings.TrimSpace(req.Objective),
		Status:      "active",
	}
	if req.Metadata != nil {
		project.Metadata = types.ObjectMetadata{}
		if tags, ok := req.Metadata["tags"].([]interface{}); ok {
			for _, t := range tags {
				if s, ok := t.(string); ok {
					project.Metadata.Tags = append(project.Metadata.Tags, s)
				}
			}
		}
		if t, ok := req.Metadata["type"].(string); ok {
			project.Metadata.Type = t
		}
		if extra, ok := req.Metadata["extra"].(map[string]interface{}); ok {
			project.Metadata.Extra = extra
		}
	}

	project.GiteaDefaultBranch = "main"

	if s.giteaClient != nil && s.giteaCfg != nil && s.giteaCfg.Enabled {
		repoName := s.buildRepoName(caller.OrgID, publicID)
		repoInfo, _, err := s.giteaClient.CreateRepo(gitea.CreateRepoOption{
			Name:        repoName,
			Description: strings.TrimSpace(req.Description),
			Private:     true,
			AutoInit:    true,
		})
		if err != nil {
			return nil, fmt.Errorf("create gitea repo: %w", err)
		}
		project.GiteaRepoFullName = repoInfo.FullName
		project.GiteaRepoID = repoInfo.ID
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := db.CreateProject(ctx, tx, project); err != nil {
			return err
		}
		participantsPayload, participantsChanged, err := s.bindProjectMembers(ctx, tx, project.ID, caller, req.Members)
		if err != nil {
			return err
		}

		activityTime := time.Now()
		if err := s.createProjectActivityAt(ctx, tx, project.PublicID, caller.Uin, types.ProjectActivityActionProjectCreated, types.ProjectActivityPayload{}, nil, activityTime); err != nil {
			return err
		}
		if participantsChanged {
			if err := s.createProjectActivityAt(ctx, tx, project.PublicID, caller.Uin, types.ProjectActivityActionParticipantsChanged, participantsPayload, nil, activityTime.Add(time.Millisecond)); err != nil {
				return err
			}
		}

		skillIDs := extractProjectSkillIDs(project.Metadata)
		if len(skillIDs) > 0 {
			payload := types.ProjectActivityPayload{
				AddedSkillIDs: skillIDs,
			}
			if err := s.createProjectActivityAt(ctx, tx, project.PublicID, caller.Uin, types.ProjectActivityActionSkillsChanged, payload, nil, activityTime.Add(2*time.Millisecond)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if project.GiteaRepoFullName != "" {
		if err := git.InitRepoStructure(ctx, s.giteaClient, project.GiteaRepoFullName); err != nil {
			logs.WarnContextf(ctx, "[project] init repo structure: %v", err)
		}
	}

	return convertToContractProject(project), nil
}

func (s *projectService) GetProject(ctx context.Context, publicID string) (*contract.Project, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(publicID) == "" {
		return nil, errors.New("public_id is required")
	}

	project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, publicID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if err := s.verifyProjectAccess(ctx, s.db, project, caller); err != nil {
		return nil, err
	}
	return convertToContractProject(project), nil
}

func (s *projectService) UpdateProject(ctx context.Context, publicID string, req *contract.UpdateProjectRequest) (*contract.Project, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(publicID) == "" {
		return nil, errors.New("public_id is required")
	}

	var project *types.Project
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project, err = db.GetProjectByPublicID(ctx, tx, caller.OrgID, publicID)
		if err != nil {
			return err
		}
		if project == nil {
			return errors.New("project not found")
		}
		if err := verifyUserPermission(project.OwnerID, caller.Uin); err != nil {
			return err
		}

		if req.Name != nil {
			project.Name = strings.TrimSpace(*req.Name)
			if project.Name == "" {
				return errors.New("name cannot be empty")
			}
		}
		if req.Description != nil {
			project.Description = strings.TrimSpace(*req.Description)
		}
		if req.Objective != nil {
			project.Objective = strings.TrimSpace(*req.Objective)
		}
		if req.OwnerID != nil {
			project.OwnerID = *req.OwnerID
		}
		if req.Status != nil {
			project.Status = *req.Status
		}
		oldSkillIDs := extractProjectSkillIDs(project.Metadata)
		if req.Metadata != nil {
			if *req.Metadata != nil {
				newMeta := types.ObjectMetadata{}
				if tags, ok := (*req.Metadata)["tags"].([]interface{}); ok {
					for _, t := range tags {
						if s, ok := t.(string); ok {
							newMeta.Tags = append(newMeta.Tags, s)
						}
					}
				}
				if t, ok := (*req.Metadata)["type"].(string); ok {
					newMeta.Type = t
				}
				if extra, ok := (*req.Metadata)["extra"].(map[string]interface{}); ok {
					newMeta.Extra = extra
				}
				project.Metadata = newMeta
			}
		}
		newSkillIDs := extractProjectSkillIDs(project.Metadata)

		if err := db.UpdateProject(ctx, tx, project); err != nil {
			return err
		}

		if len(req.Members) > 0 {
			payload, changed, err := s.syncProjectMembers(ctx, tx, project.ID, caller.OrgID, caller.Uin, req.Members)
			if err != nil {
				return err
			}
			if changed {
				if err := s.createProjectActivity(ctx, tx, project.PublicID, caller.Uin, types.ProjectActivityActionParticipantsChanged, payload, nil); err != nil {
					return err
				}
			}
		}

		if req.Metadata != nil {
			addedSkillIDs, removedSkillIDs := diffStringSlices(oldSkillIDs, newSkillIDs)
			if len(addedSkillIDs) > 0 || len(removedSkillIDs) > 0 {
				payload := types.ProjectActivityPayload{
					AddedSkillIDs:   addedSkillIDs,
					RemovedSkillIDs: removedSkillIDs,
				}
				if err := s.createProjectActivity(ctx, tx, project.PublicID, caller.Uin, types.ProjectActivityActionSkillsChanged, payload, nil); err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return convertToContractProject(project), nil
}

func (s *projectService) DeleteProject(ctx context.Context, publicID string) error {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(publicID) == "" {
		return errors.New("public_id is required")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project, err := db.GetProjectByPublicID(ctx, tx, caller.OrgID, publicID)
		if err != nil {
			return err
		}
		if project == nil {
			return errors.New("project not found")
		}
		if err := verifyUserPermission(project.OwnerID, caller.Uin); err != nil {
			return err
		}
		if err := db.DeleteTasksByProjectID(ctx, tx, caller.OrgID, project.ID); err != nil {
			return err
		}
		return db.DeleteProject(ctx, tx, project.ID)
	})
}

func (s *projectService) ListProjects(ctx context.Context, req *contract.ListProjectsRequest) (*contract.ProjectList, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	req.Fill()

	projectIDs, err := db.ListProjectIDsByUser(ctx, s.db, caller.OrgID, caller.Uin)
	if err != nil {
		return nil, err
	}
	if len(projectIDs) == 0 {
		return &contract.ProjectList{
			Total:  0,
			Offset: req.Offset,
			Limit:  req.Limit,
			Items:  []contract.Project{},
		}, nil
	}

	opt := types.NewPageQuery(*caller, req.Offset, req.Limit)
	opt.ProjectIDs = projectIDs
	opt.ListAll = req.ListAll
	if req.Keyword != nil && *req.Keyword != "" {
		opt.AddFilter("name", *req.Keyword)
	}
	if req.Status != nil && *req.Status != "" {
		opt.AddFilter("status", *req.Status)
	}

	projects, total, err := db.ListProjects(ctx, s.db, opt)
	if err != nil {
		return nil, err
	}

	projIDsForCount := make([]uint, 0, len(projects))
	for _, project := range projects {
		projIDsForCount = append(projIDsForCount, project.ID)
	}
	var taskCountMap map[uint]int64
	if len(projIDsForCount) > 0 {
		taskCountMap, err = db.CountTasksByProjectIDs(ctx, s.db, caller.OrgID, projIDsForCount)
		if err != nil {
			return nil, err
		}
	}

	items := make([]contract.Project, 0, len(projects))
	for _, project := range projects {
		item := convertToContractProject(project)
		item.TaskCount = taskCountMap[project.ID]
		items = append(items, *item)
	}
	return &contract.ProjectList{
		Total:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Items:  items,
	}, nil
}

func (s *projectService) ListProjectActivities(ctx context.Context, req *contract.ListProjectActivitiesRequest) (*contract.ProjectActivityList, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &contract.ListProjectActivitiesRequest{}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var beforeTime *time.Time
	var beforeID uint
	if strings.TrimSpace(req.Cursor) != "" {
		cursor, err := decodeProjectActivityCursor(req.Cursor)
		if err != nil {
			return nil, err
		}
		beforeTime = &cursor.CreatedAt
		beforeID = cursor.ID
	}

	opt := db.ProjectActivityListOptions{
		OperatorID: req.OperatorID,
		BeforeTime: beforeTime,
		BeforeID:   beforeID,
		Limit:      limit,
	}

	if strings.TrimSpace(req.ProjectID) != "" {
		project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, strings.TrimSpace(req.ProjectID))
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, errors.New("project not found")
		}
		if err := s.verifyProjectAccess(ctx, s.db, project, caller); err != nil {
			return nil, err
		}
		opt.ProjectID = project.PublicID
	} else {
		projectIDs, err := db.ListProjectIDsByUser(ctx, s.db, caller.OrgID, caller.Uin)
		if err != nil {
			return nil, err
		}
		projects, err := db.GetProjectsByIDs(ctx, s.db, projectIDs)
		if err != nil {
			return nil, err
		}
		opt.ProjectIDs = make([]string, 0, len(projects))
		for _, project := range projects {
			opt.ProjectIDs = append(opt.ProjectIDs, project.PublicID)
		}
	}

	activities, err := db.ListProjectActivities(ctx, s.db, opt)
	if err != nil {
		return nil, err
	}
	items, err := s.buildProjectActivityItems(ctx, caller.OrgID, activities)
	if err != nil {
		return nil, err
	}

	nextCursor := ""
	if len(activities) == limit {
		last := activities[len(activities)-1]
		nextCursor = encodeProjectActivityCursor(projectActivityCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return &contract.ProjectActivityList{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}

func (s *projectService) GetWorkbenchRecentContext(ctx context.Context) (*contract.WorkbenchRecentContext, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}

	recent, err := db.GetWorkbenchRecentContext(ctx, s.db, caller.OrgID, caller.Uin)
	if err != nil {
		return nil, err
	}
	if recent == nil {
		return nil, nil
	}

	project, err := db.GetProjectByID(ctx, s.db, recent.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil || project.OrgID != caller.OrgID || s.verifyProjectAccess(ctx, s.db, project, caller) != nil {
		return nil, nil
	}

	var task *types.Task
	if recent.TaskID != nil {
		task, err = db.GetTaskByID(ctx, s.db, caller.OrgID, *recent.TaskID)
		if err != nil {
			return nil, err
		}
		if task == nil || task.ProjectID != project.ID || verifyUserPermission(task.OwnerID, caller.Uin) != nil {
			task = nil
		}
	}

	return buildWorkbenchRecentContext(project, task, recent.UsedAt), nil
}

func (s *projectService) SaveWorkbenchRecentContext(ctx context.Context, req *contract.SaveWorkbenchRecentContextRequest) (*contract.WorkbenchRecentContext, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		return nil, errors.New("project_id is required")
	}

	project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, strings.TrimSpace(req.ProjectID))
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if err := s.verifyProjectAccess(ctx, s.db, project, caller); err != nil {
		return nil, err
	}

	var task *types.Task
	var taskID *uint
	if req.TaskID != nil && strings.TrimSpace(*req.TaskID) != "" {
		task, err = db.GetTaskByPublicID(ctx, s.db, caller.OrgID, strings.TrimSpace(*req.TaskID))
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, errors.New("task not found")
		}
		if err := verifyUserPermission(task.OwnerID, caller.Uin); err != nil {
			return nil, err
		}
		if task.ProjectID != project.ID {
			return nil, errors.New("task does not belong to project")
		}
		taskID = &task.ID
	}

	usedAt := time.Now()
	entity := &types.WorkbenchRecentContext{
		OrgID:     caller.OrgID,
		Uin:       caller.Uin,
		ProjectID: project.ID,
		TaskID:    taskID,
		UsedAt:    usedAt,
	}
	if err := db.UpsertWorkbenchRecentContext(ctx, s.db, entity); err != nil {
		return nil, err
	}

	return buildWorkbenchRecentContext(project, task, usedAt), nil
}

// bindProjectMembers 创建项目时绑定默认 AI 队友 + 用户指定的额外 AI 队友 + 创建者本人。
func (s *projectService) bindProjectMembers(ctx context.Context, tx *gorm.DB, projectID uint, caller *types.Caller, inputs []contract.MemberInput) (types.ProjectActivityPayload, bool, error) {
	payload := types.ProjectActivityPayload{}
	defaultAssistantID, err := db.GetDefaultAssistantIDByOrg(ctx, tx, caller.OrgID)
	if err != nil {
		return payload, false, fmt.Errorf("get default assistant: %w", err)
	}
	if defaultAssistantID == 0 {
		return payload, false, ErrNoDefaultAssistantInOrg
	}

	assistantPublicIDs, userPublicIDs, err := parseMemberInputs(inputs)
	if err != nil {
		return payload, false, err
	}

	assistantIDs, err := resolveAssistantIDsByPublicID(ctx, tx, caller.OrgID, assistantPublicIDs)
	if err != nil {
		return payload, false, fmt.Errorf("resolve assistant public ids: %w", err)
	}

	if err := validateAssistantIDs(assistantIDs, defaultAssistantID); err != nil {
		return payload, false, err
	}

	userUins, err := db.GetUinsByPublicIDs(ctx, tx, caller.OrgID, userPublicIDs)
	if err != nil {
		return payload, false, fmt.Errorf("get user uins: %w", err)
	}

	var members []*types.ProjectMember
	now := time.Now()

	members = append(members, &types.ProjectMember{
		ProjectID:  projectID,
		MemberID:   defaultAssistantID,
		MemberType: types.MemberTypeAssistant,
		MemberRole: types.MemberRoleMember,
		IsDefault:  true,
		JoinedAt:   now,
	})

	for _, id := range assistantIDs {
		members = append(members, &types.ProjectMember{
			ProjectID:  projectID,
			MemberID:   id,
			MemberType: types.MemberTypeAssistant,
			MemberRole: types.MemberRoleMember,
			IsDefault:  false,
			JoinedAt:   now,
		})
	}

	members = append(members, &types.ProjectMember{
		ProjectID:  projectID,
		MemberID:   caller.Uin,
		MemberType: types.MemberTypeUser,
		MemberRole: types.MemberRoleOwner,
		JoinedAt:   now,
	})

	for _, uin := range userUins {
		if uin == caller.Uin {
			continue
		}
		members = append(members, &types.ProjectMember{
			ProjectID:  projectID,
			MemberID:   uin,
			MemberType: types.MemberTypeUser,
			MemberRole: types.MemberRoleMember,
			JoinedAt:   now,
		})
	}

	if err := db.BatchCreateProjectMembers(ctx, tx, members); err != nil {
		return payload, false, err
	}

	addedAssistantIDs := make([]uint, 0, len(assistantIDs))
	for _, id := range assistantIDs {
		if id == defaultAssistantID {
			continue
		}
		addedAssistantIDs = append(addedAssistantIDs, id)
	}
	payload.AddedAITeammateIDs, err = publicIDsForAssistants(ctx, tx, uniqueUintSlice(addedAssistantIDs))
	if err != nil {
		return payload, false, err
	}
	addedUserIDs := make([]uint, 0, len(userUins))
	for _, uin := range userUins {
		if uin == caller.Uin {
			continue
		}
		addedUserIDs = append(addedUserIDs, uin)
	}
	payload.AddedMemberIDs, err = publicIDsForUsers(ctx, tx, uniqueUintSlice(addedUserIDs))
	if err != nil {
		return payload, false, err
	}

	changed := len(payload.AddedMemberIDs) > 0 || len(payload.AddedAITeammateIDs) > 0
	return payload, changed, nil
}

// parseMemberInputs 将 MemberInput 列表拆分为 assistant public_id 和 user public_id 两组。
func parseMemberInputs(inputs []contract.MemberInput) (assistantPublicIDs []string, userPublicIDs []string, err error) {
	for _, m := range inputs {
		switch m.Type {
		case "assistant":
			assistantPublicIDs = append(assistantPublicIDs, m.ID)
		case "user":
			userPublicIDs = append(userPublicIDs, m.ID)
		default:
			return nil, nil, fmt.Errorf("invalid member type: %s", m.Type)
		}
	}
	return assistantPublicIDs, userPublicIDs, nil
}

// syncProjectMembers 在 UpdateProject 时 diff 当前成员与传入列表：
// 新增的添加，要移除的删除（is_default=true 的不可移除）。
func (s *projectService) syncProjectMembers(ctx context.Context, tx *gorm.DB, projectID, orgID, callerUin uint, inputs []contract.MemberInput) (types.ProjectActivityPayload, bool, error) {
	payload := types.ProjectActivityPayload{}
	defaultAssistantID, err := db.GetDefaultAssistantIDByOrg(ctx, tx, orgID)
	if err != nil {
		return payload, false, fmt.Errorf("get default assistant: %w", err)
	}
	if defaultAssistantID == 0 {
		return payload, false, ErrNoDefaultAssistantInOrg
	}

	assistantPublicIDs, userPublicIDs, err := parseMemberInputs(inputs)
	if err != nil {
		return payload, false, err
	}

	assistantIDs, err := resolveAssistantIDsByPublicID(ctx, tx, orgID, assistantPublicIDs)
	if err != nil {
		return payload, false, fmt.Errorf("resolve assistant public ids: %w", err)
	}

	if err := validateAssistantIDs(assistantIDs, defaultAssistantID); err != nil {
		return payload, false, err
	}

	userUins, err := db.GetUinsByPublicIDs(ctx, tx, orgID, userPublicIDs)
	if err != nil {
		return payload, false, fmt.Errorf("get user uins: %w", err)
	}

	now := time.Now()

	// 同步 AI 队友
	existingAssistants, err := db.ListProjectAssistantMembers(ctx, tx, projectID)
	if err != nil {
		return payload, false, fmt.Errorf("list project assistants: %w", err)
	}
	existingNonDefault := make(map[uint]*types.ProjectMember)
	for _, m := range existingAssistants {
		if m.IsDefault {
			continue
		}
		existingNonDefault[m.MemberID] = m
	}
	requestedAssistantSet := make(map[uint]bool, len(assistantIDs))
	for _, id := range assistantIDs {
		requestedAssistantSet[id] = true
	}
	oldAssistantIDs := make([]uint, 0, len(existingNonDefault))
	for id := range existingNonDefault {
		oldAssistantIDs = append(oldAssistantIDs, id)
	}
	addedAssistantIDs, removedAssistantIDs := diffUintSlices(oldAssistantIDs, assistantIDs)
	payload.AddedAITeammateIDs, err = publicIDsForAssistants(ctx, tx, addedAssistantIDs)
	if err != nil {
		return payload, false, err
	}
	payload.RemovedAITeammateIDs, err = publicIDsForAssistants(ctx, tx, removedAssistantIDs)
	if err != nil {
		return payload, false, err
	}
	for _, m := range existingNonDefault {
		if !requestedAssistantSet[m.MemberID] {
			if err := db.DeleteProjectMember(ctx, tx, m.ID); err != nil {
				return payload, false, fmt.Errorf("delete project assistant member %d: %w", m.MemberID, err)
			}
		}
	}
	for _, id := range assistantIDs {
		if _, ok := existingNonDefault[id]; !ok {
			if err := db.CreateProjectMember(ctx, tx, &types.ProjectMember{
				ProjectID:  projectID,
				MemberID:   id,
				MemberType: types.MemberTypeAssistant,
				MemberRole: types.MemberRoleMember,
				IsDefault:  false,
				JoinedAt:   now,
			}); err != nil {
				return payload, false, fmt.Errorf("create project assistant member %d: %w", id, err)
			}
		}
	}

	// 同步用户成员
	existingUsers, err := db.ListProjectMemberByType(ctx, tx, projectID, types.MemberTypeUser)
	if err != nil {
		return payload, false, fmt.Errorf("list project user members: %w", err)
	}
	existingUserMap := make(map[uint]*types.ProjectMember)
	for _, m := range existingUsers {
		if m.MemberRole == types.MemberRoleOwner {
			continue
		}
		existingUserMap[m.MemberID] = m
	}
	requestedUserSet := make(map[uint]bool, len(userUins))
	requestedUserIDs := make([]uint, 0, len(userUins))
	for _, uin := range userUins {
		if uin == callerUin {
			continue
		}
		requestedUserSet[uin] = true
		requestedUserIDs = append(requestedUserIDs, uin)
	}
	oldUserIDs := make([]uint, 0, len(existingUserMap))
	for id := range existingUserMap {
		oldUserIDs = append(oldUserIDs, id)
	}
	addedUserIDs, removedUserIDs := diffUintSlices(oldUserIDs, requestedUserIDs)
	payload.AddedMemberIDs, err = publicIDsForUsers(ctx, tx, addedUserIDs)
	if err != nil {
		return payload, false, err
	}
	payload.RemovedMemberIDs, err = publicIDsForUsers(ctx, tx, removedUserIDs)
	if err != nil {
		return payload, false, err
	}
	for _, m := range existingUserMap {
		if !requestedUserSet[m.MemberID] {
			if err := db.DeleteProjectMember(ctx, tx, m.ID); err != nil {
				return payload, false, fmt.Errorf("delete project user member %d: %w", m.MemberID, err)
			}
		}
	}
	for _, uin := range userUins {
		if uin == callerUin {
			continue
		}
		if _, ok := existingUserMap[uin]; !ok {
			if err := db.CreateProjectMember(ctx, tx, &types.ProjectMember{
				ProjectID:  projectID,
				MemberID:   uin,
				MemberType: types.MemberTypeUser,
				MemberRole: types.MemberRoleMember,
				JoinedAt:   now,
			}); err != nil {
				return payload, false, fmt.Errorf("create project user member %d: %w", uin, err)
			}
		}
	}

	changed := len(payload.AddedMemberIDs) > 0 ||
		len(payload.RemovedMemberIDs) > 0 ||
		len(payload.AddedAITeammateIDs) > 0 ||
		len(payload.RemovedAITeammateIDs) > 0
	return payload, changed, nil
}

// verifyProjectAccess 校验调用方是否有权限访问项目（owner 或成员）。
func (s *projectService) verifyProjectAccess(ctx context.Context, dbConn *gorm.DB, project *types.Project, caller *types.Caller) error {
	if project.OwnerID == caller.Uin {
		return nil
	}
	isMember, err := db.IsProjectUserMember(ctx, dbConn, caller.OrgID, caller.Uin, project.ID)
	if err != nil {
		return err
	}
	if !isMember {
		return errors.New("permission denied")
	}
	return nil
}

func (s *projectService) createProjectActivity(
	ctx context.Context,
	tx *gorm.DB,
	projectID string,
	operatorUin uint,
	actionType types.ProjectActivityAction,
	payload types.ProjectActivityPayload,
	requestID *string,
) error {
	return s.createProjectActivityAt(ctx, tx, projectID, operatorUin, actionType, payload, requestID, time.Now())
}

func (s *projectService) createProjectActivityAt(
	ctx context.Context,
	tx *gorm.DB,
	projectID string,
	operatorUin uint,
	actionType types.ProjectActivityAction,
	payload types.ProjectActivityPayload,
	requestID *string,
	createdAt time.Time,
) error {
	operatorID, err := publicIDForUser(ctx, tx, operatorUin)
	if err != nil {
		return err
	}
	payload = normalizeProjectActivityPayload(payload)
	return db.CreateProjectActivity(ctx, tx, &types.ProjectActivity{
		ProjectID:  projectID,
		OperatorID: operatorID,
		ActionType: actionType,
		Payload:    payload,
		RequestID:  requestID,
		Version:    1,
		CreatedAt:  createdAt,
	})
}

func normalizeProjectActivityPayload(payload types.ProjectActivityPayload) types.ProjectActivityPayload {
	if payload.AddedSkillIDs == nil {
		payload.AddedSkillIDs = []string{}
	}
	if payload.RemovedSkillIDs == nil {
		payload.RemovedSkillIDs = []string{}
	}
	if payload.AddedMemberIDs == nil {
		payload.AddedMemberIDs = []string{}
	}
	if payload.RemovedMemberIDs == nil {
		payload.RemovedMemberIDs = []string{}
	}
	if payload.AddedAITeammateIDs == nil {
		payload.AddedAITeammateIDs = []string{}
	}
	if payload.RemovedAITeammateIDs == nil {
		payload.RemovedAITeammateIDs = []string{}
	}
	return payload
}

func (s *projectService) buildProjectActivityItems(ctx context.Context, orgID uint, activities []*types.ProjectActivity) ([]contract.ProjectActivityItem, error) {
	if len(activities) == 0 {
		return []contract.ProjectActivityItem{}, nil
	}

	userIDs := make([]string, 0, len(activities))
	assistantIDs := make([]string, 0)
	skillIDs := make([]string, 0)
	for _, activity := range activities {
		userIDs = append(userIDs, activity.OperatorID)
		payload := normalizeProjectActivityPayload(activity.Payload)
		userIDs = append(userIDs, payload.AddedMemberIDs...)
		userIDs = append(userIDs, payload.RemovedMemberIDs...)
		assistantIDs = append(assistantIDs, payload.AddedAITeammateIDs...)
		assistantIDs = append(assistantIDs, payload.RemovedAITeammateIDs...)
		skillIDs = append(skillIDs, payload.AddedSkillIDs...)
		skillIDs = append(skillIDs, payload.RemovedSkillIDs...)
	}

	users, err := db.GetUsersByPublicIDs(ctx, s.db, uniqueStringSlice(userIDs))
	if err != nil {
		return nil, err
	}
	userMap := make(map[string]*types.User, len(users))
	for _, user := range users {
		userMap[user.PublicID] = user
	}

	assistants, err := db.GetAssistantsByPublicIDs(ctx, s.db, uniqueStringSlice(assistantIDs))
	if err != nil {
		return nil, err
	}
	assistantMap := make(map[string]*types.DigitalAssistant, len(assistants))
	for _, assistant := range assistants {
		assistantMap[assistant.PublicID] = assistant
	}

	skillMap, err := s.buildProjectActivitySkillMap(ctx, orgID, skillIDs)
	if err != nil {
		return nil, err
	}

	items := make([]contract.ProjectActivityItem, 0, len(activities))
	for _, activity := range activities {
		payload := normalizeProjectActivityPayload(activity.Payload)
		item := contract.ProjectActivityItem{
			ID:         activity.ID,
			ProjectID:  activity.ProjectID,
			OperatorID: activity.OperatorID,
			Operator:   userActorFromMap(userMap, activity.OperatorID),
			ActionType: string(activity.ActionType),
			Payload: contract.ProjectActivityPayloadView{
				AddedSkills:        skillRefsFromMap(skillMap, payload.AddedSkillIDs),
				RemovedSkills:      skillRefsFromMap(skillMap, payload.RemovedSkillIDs),
				AddedMembers:       userRefsFromMap(userMap, payload.AddedMemberIDs),
				RemovedMembers:     userRefsFromMap(userMap, payload.RemovedMemberIDs),
				AddedAITeammates:   assistantRefsFromMap(assistantMap, payload.AddedAITeammateIDs),
				RemovedAITeammates: assistantRefsFromMap(assistantMap, payload.RemovedAITeammateIDs),
			},
			CreatedAt: activity.CreatedAt,
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *projectService) buildProjectActivitySkillMap(ctx context.Context, orgID uint, skillIDs []string) (map[string]contract.ProjectActivitySkill, error) {
	ids := uniqueStringSlice(skillIDs)
	result := make(map[string]contract.ProjectActivitySkill, len(ids))
	for _, id := range ids {
		result[id] = contract.ProjectActivitySkill{ID: id}
	}
	if len(ids) == 0 {
		return result, nil
	}

	skills, err := db.GetSkillsByCodes(ctx, s.db, orgID, ids)
	if err != nil {
		return nil, err
	}
	for _, skill := range skills {
		result[skill.Code] = contract.ProjectActivitySkill{
			ID:   skill.Code,
			Name: skill.Name,
			Icon: skill.Icon,
		}
	}

	items, err := db.GetSkillMarketplaceItemsBySkillIDs(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if existing := result[item.SkillID]; existing.Name != "" {
			continue
		}
		name := item.TranslatedName
		if strings.TrimSpace(name) == "" {
			name = item.Name
		}
		result[item.SkillID] = contract.ProjectActivitySkill{
			ID:   item.SkillID,
			Name: name,
		}
	}
	return result, nil
}

func publicIDForUser(ctx context.Context, database *gorm.DB, uin uint) (string, error) {
	user, err := db.GetUserByUin(ctx, database, uin)
	if err != nil {
		return "", err
	}
	if user == nil || strings.TrimSpace(user.PublicID) == "" {
		return "", fmt.Errorf("user %d public_id not found", uin)
	}
	return user.PublicID, nil
}

func publicIDsForUsers(ctx context.Context, database *gorm.DB, ids []uint) ([]string, error) {
	if len(ids) == 0 {
		return []string{}, nil
	}
	userMap, err := db.GetUsersByUins(ctx, database, uniqueUintSlice(ids))
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		user := userMap[id]
		publicID := ""
		if user != nil {
			publicID = strings.TrimSpace(user.PublicID)
		}
		if publicID == "" {
			return nil, fmt.Errorf("user %d public_id not found", id)
		}
		result = append(result, publicID)
	}
	return result, nil
}

func publicIDsForAssistants(ctx context.Context, database *gorm.DB, ids []uint) ([]string, error) {
	if len(ids) == 0 {
		return []string{}, nil
	}
	assistants, err := db.GetAssistantsByIDs(ctx, database, uniqueUintSlice(ids))
	if err != nil {
		return nil, err
	}
	publicIDByID := make(map[uint]string, len(assistants))
	for _, assistant := range assistants {
		publicIDByID[assistant.ID] = assistant.PublicID
	}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		publicID := strings.TrimSpace(publicIDByID[id])
		if publicID == "" {
			return nil, fmt.Errorf("digital assistant %d public_id not found", id)
		}
		result = append(result, publicID)
	}
	return result, nil
}

func userActorFromMap(users map[string]*types.User, id string) *contract.ProjectActivityActor {
	user, ok := users[id]
	if !ok {
		return &contract.ProjectActivityActor{ID: id}
	}
	return &contract.ProjectActivityActor{
		ID:        id,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
	}
}

func userRefsFromMap(users map[string]*types.User, ids []string) []contract.ProjectActivityActor {
	refs := make([]contract.ProjectActivityActor, 0, len(ids))
	for _, id := range ids {
		actor := userActorFromMap(users, id)
		refs = append(refs, *actor)
	}
	return refs
}

func assistantRefsFromMap(assistants map[string]*types.DigitalAssistant, ids []string) []contract.ProjectActivityActor {
	refs := make([]contract.ProjectActivityActor, 0, len(ids))
	for _, id := range ids {
		ref := contract.ProjectActivityActor{ID: id}
		if assistant, ok := assistants[id]; ok {
			ref.Name = assistant.Name
			ref.AvatarURL = assistant.Avatar
		}
		refs = append(refs, ref)
	}
	return refs
}

func skillRefsFromMap(skills map[string]contract.ProjectActivitySkill, ids []string) []contract.ProjectActivitySkill {
	refs := make([]contract.ProjectActivitySkill, 0, len(ids))
	for _, id := range ids {
		if skill, ok := skills[id]; ok {
			refs = append(refs, skill)
			continue
		}
		refs = append(refs, contract.ProjectActivitySkill{ID: id})
	}
	return refs
}

func encodeProjectActivityCursor(cursor projectActivityCursor) string {
	data, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeProjectActivityCursor(value string) (projectActivityCursor, error) {
	var cursor projectActivityCursor
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return cursor, errors.New("invalid cursor")
	}
	if err := json.Unmarshal(data, &cursor); err != nil {
		return cursor, errors.New("invalid cursor")
	}
	if cursor.ID == 0 || cursor.CreatedAt.IsZero() {
		return cursor, errors.New("invalid cursor")
	}
	return cursor, nil
}

func extractProjectSkillIDs(meta types.ObjectMetadata) []string {
	if meta.Extra == nil {
		return []string{}
	}
	rawSkills, ok := meta.Extra["skills"]
	if !ok || rawSkills == nil {
		return []string{}
	}
	skillsSlice, ok := rawSkills.([]interface{})
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(skillsSlice))
	for _, item := range skillsSlice {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		code, _ := entry["code"].(string)
		code = strings.TrimSpace(code)
		if code == "" {
			name, _ := entry["name"].(string)
			code = strings.TrimSpace(name)
		}
		if code != "" {
			result = append(result, code)
		}
	}
	return uniqueStringSlice(result)
}

func diffStringSlices(oldIDs, newIDs []string) (added, removed []string) {
	oldSet := make(map[string]bool, len(oldIDs))
	newSet := make(map[string]bool, len(newIDs))
	for _, id := range oldIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			oldSet[id] = true
		}
	}
	for _, id := range newIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			newSet[id] = true
			if !oldSet[id] {
				added = append(added, id)
			}
		}
	}
	for _, id := range oldIDs {
		id = strings.TrimSpace(id)
		if id != "" && !newSet[id] {
			removed = append(removed, id)
		}
	}
	return added, removed
}

func diffUintSlices(oldIDs, newIDs []uint) (added, removed []uint) {
	oldSet := make(map[uint]bool, len(oldIDs))
	newSet := make(map[uint]bool, len(newIDs))
	for _, id := range oldIDs {
		if id > 0 {
			oldSet[id] = true
		}
	}
	for _, id := range newIDs {
		if id == 0 {
			continue
		}
		newSet[id] = true
		if !oldSet[id] {
			added = append(added, id)
		}
	}
	for _, id := range oldIDs {
		if id > 0 && !newSet[id] {
			removed = append(removed, id)
		}
	}
	return added, removed
}

func uniqueUintSlice(values []uint) []uint {
	seen := make(map[uint]bool, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func uniqueStringSlice(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

// validateAssistantIDs 校验 assistant ID 列表：去重、非零、不出现在默认 assistant 中。
func validateAssistantIDs(ids []uint, defaultAssistantID uint) error {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]bool, len(ids))
	for _, id := range ids {
		if id == 0 {
			return ErrInvalidAssistantID
		}
		if id == defaultAssistantID {
			return fmt.Errorf("default assistant %d cannot be specified as extra", id)
		}
		if seen[id] {
			return fmt.Errorf("%w: %d", ErrDuplicateAssistant, id)
		}
		seen[id] = true
	}
	return nil
}

func convertToContractProject(project *types.Project) *contract.Project {
	if project == nil {
		return nil
	}

	var metadata map[string]interface{}
	m := make(map[string]interface{})
	if len(project.Metadata.Tags) > 0 {
		m["tags"] = project.Metadata.Tags
	}
	if project.Metadata.Type != "" {
		m["type"] = project.Metadata.Type
	}
	if project.Metadata.Extra != nil && len(project.Metadata.Extra) > 0 {
		m["extra"] = project.Metadata.Extra
	}
	if len(m) > 0 {
		metadata = m
	}

	return &contract.Project{
		PublicID:    project.PublicID,
		Name:        project.Name,
		Description: project.Description,
		Objective:   project.Objective,
		OwnerID:     project.OwnerID,
		Status:      project.Status,
		Metadata:    metadata,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}
}

func buildWorkbenchRecentContext(project *types.Project, task *types.Task, usedAt time.Time) *contract.WorkbenchRecentContext {
	if project == nil {
		return nil
	}

	var taskID *string
	var taskTitle *string
	if task != nil {
		// 中文注释：任务为空表示用户最近只选中了项目，首页应回显为“新建任务”入口。
		taskID = &task.PublicID
		taskTitle = &task.Title
	}

	return &contract.WorkbenchRecentContext{
		ProjectID:   project.PublicID,
		ProjectName: project.Name,
		TaskID:      taskID,
		TaskTitle:   taskTitle,
		UsedAt:      usedAt,
	}
}

func (s *projectService) DetailProject(ctx context.Context, publicID string) (*contract.ProjectDetail, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(publicID) == "" {
		return nil, errors.New("public_id is required")
	}

	project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, publicID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if err := s.verifyProjectAccess(ctx, s.db, project, caller); err != nil {
		return nil, err
	}

	result := &contract.ProjectDetail{
		Project: *convertToContractProject(project),
		Tasks:   make([]contract.ProjectTaskItem, 0),
		Members: make([]contract.ProjectMemberItem, 0),
	}

	// 查询项目会话
	prjSession, _ := db.GetProjectSession(ctx, s.db, project.ID)
	if prjSession != nil {
		result.Session = convertToContractSession(prjSession, s.db)
	}

	// 查询项目任务
	tasks, err := db.ListTasksByProjectID(ctx, s.db, caller.OrgID, project.ID)
	if err != nil {
		return nil, err
	}

	// 收集任务会话ID，批量查询会话
	taskSessionIDs := make([]uint, 0)
	taskIDs := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		taskIDs = append(taskIDs, t.ID)
		if t.SessionID != nil {
			taskSessionIDs = append(taskSessionIDs, *t.SessionID)
		}
	}

	taskSessions, err := db.GetSessionsByIDs(ctx, s.db, taskSessionIDs)
	if err != nil {
		return nil, err
	}
	sessionMap := make(map[uint]*types.Session, len(taskSessions))
	for _, sess := range taskSessions {
		sessionMap[sess.ID] = sess
	}

	for _, t := range tasks {
		item := contract.ProjectTaskItem{
			Task: *convertToContractTask(t, project.PublicID, project.Name),
		}
		if t.SessionID != nil {
			if sess, ok := sessionMap[*t.SessionID]; ok {
				item.Session = convertToContractSession(sess, s.db)
			}
		}
		result.Tasks = append(result.Tasks, item)
	}

	// 查询项目成员
	members, err := db.ListProjectMembers(ctx, s.db, project.ID)
	if err != nil {
		return nil, err
	}

	userIDs := make([]uint, 0)
	assistantIDs := make([]uint, 0)
	for _, m := range members {
		if m.MemberType == types.MemberTypeUser {
			userIDs = append(userIDs, m.MemberID)
		} else if m.MemberType == types.MemberTypeAssistant {
			assistantIDs = append(assistantIDs, m.MemberID)
		}
	}

	users, _ := db.GetUsersByIDs(ctx, s.db, userIDs)
	userMap := make(map[uint]*types.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	assistants, _ := db.GetAssistantsByIDs(ctx, s.db, assistantIDs)
	assistantMap := make(map[uint]*types.DigitalAssistant, len(assistants))
	for _, a := range assistants {
		assistantMap[a.ID] = a
	}

	for _, m := range members {
		item := contract.ProjectMemberItem{
			MemberID:   m.MemberID,
			MemberType: string(m.MemberType),
			MemberRole: string(m.MemberRole),
			IsDefault:  m.IsDefault,
			JoinedAt:   m.JoinedAt,
		}
		if m.MemberType == types.MemberTypeUser {
			if u, ok := userMap[m.MemberID]; ok {
				item.Name = u.Name
				item.AvatarURL = u.AvatarURL
			}
		} else if m.MemberType == types.MemberTypeAssistant {
			if a, ok := assistantMap[m.MemberID]; ok {
				item.Name = a.Name
				item.AvatarURL = a.Avatar
			}
		}
		result.Members = append(result.Members, item)
	}

	return result, nil
}

func (s *projectService) GetProjectMemory(ctx context.Context, publicID string) (*contract.ProjectMemory, error) {
	// 1. 鉴权
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(publicID) == "" {
		return nil, errors.New("public_id is required")
	}

	// 2. 查项目（org 隔离）
	project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, publicID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if err := s.verifyProjectAccess(ctx, s.db, project, caller); err != nil {
		return nil, err
	}

	// 3. 拼 repo 路径: {workspaceRoot}/projects/{orgID}/{publicID}/repo/
	workerID, err := resolveProjectWorkerID(ctx, s.db, project.OrgID, project.ID, s.inferrer)
	if err != nil {
		return nil, fmt.Errorf("resolve project worker: %w", err)
	}
	repoDir, err := workspace.ProjectRepoPath(project.OrgID, workerID, publicID)
	if err != nil {
		return nil, err
	}

	// 4. 读取 MEMORY.md
	memoryPath := workspace.ProjectMemoryPath(repoDir)
	entries, err := localmemory.ReadEntries(memoryPath)
	if err != nil {
		// 文件不存在或不可读时返回空列表而非报错
		if os.IsNotExist(err) {
			return &contract.ProjectMemory{
				Entries: []string{},
				Total:   0,
			}, nil
		}
		return nil, fmt.Errorf("read project memory: %w", err)
	}

	if entries == nil {
		entries = []string{}
	}

	return &contract.ProjectMemory{
		Entries: entries,
		Total:   len(entries),
	}, nil
}

func (s *projectService) GetProjectFileTree(ctx context.Context, publicID string, resourceType string, taskPublicID string) ([]*contract.FileTreeNode, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(publicID) == "" {
		return nil, errors.New("public_id is required")
	}

	project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, publicID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if err := s.verifyProjectAccess(ctx, s.db, project, caller); err != nil {
		return nil, err
	}

	var files []types.ProjectFile
	if taskPublicID != "" {
		task, err := db.GetTaskByPublicID(ctx, s.db, caller.OrgID, taskPublicID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, errors.New("task not found")
		}
		if task.ProjectID != project.ID {
			return nil, errors.New("task does not belong to this project")
		}
		files, err = db.ListProjectFilesByTask(ctx, s.db, caller.OrgID, project.ID, task.ID, resourceType)
		if err != nil {
			return nil, fmt.Errorf("list project files by task: %w", err)
		}
	} else {
		files, err = db.ListProjectFiles(ctx, s.db, caller.OrgID, project.ID, resourceType)
		if err != nil {
			return nil, fmt.Errorf("list project files: %w", err)
		}
	}

	return buildFileTreeFromProjectFiles(ctx, s.db, files), nil
}

// DownloadProjectFile 通过 project_file 表和 filestore 下载/预览项目文件。
func (s *projectService) DownloadProjectFile(ctx context.Context, publicID string, filePath string) (io.ReadCloser, string, int64, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	if strings.TrimSpace(publicID) == "" {
		return nil, "", 0, errors.New("public_id is required")
	}
	if strings.TrimSpace(filePath) == "" {
		return nil, "", 0, errors.New("file path is required")
	}

	project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, publicID)
	if err != nil {
		return nil, "", 0, err
	}
	if project == nil {
		return nil, "", 0, errors.New("project not found")
	}
	if err := s.verifyProjectAccess(ctx, s.db, project, caller); err != nil {
		return nil, "", 0, err
	}

	if !isPathAllowed(filePath) {
		return nil, "", 0, errors.New("file access denied")
	}

	files, err := db.ListProjectFiles(ctx, s.db, caller.OrgID, project.ID, "")
	if err != nil {
		return nil, "", 0, fmt.Errorf("list project files: %w", err)
	}

	fileName := filepath.Base(filePath)
	var target *types.ProjectFile
	for i := range files {
		fileUpload, err := db.GetFileUploadByPublicID(ctx, s.db, caller.OrgID, files[i].FilePublicID)
		if err != nil {
			return nil, "", 0, fmt.Errorf("get file upload: %w", err)
		}
		if fileUpload != nil && (fileUpload.OriginalName == fileName || fileUpload.Filename == fileName) {
			target = &files[i]
			break
		}
	}
	if target == nil {
		return nil, "", 0, fmt.Errorf("file %q not found in project files", fileName)
	}

	fileUpload, err := db.GetFileUploadByPublicID(ctx, s.db, caller.OrgID, target.FilePublicID)
	if err != nil {
		return nil, "", 0, fmt.Errorf("get file upload: %w", err)
	}
	if fileUpload == nil {
		return nil, "", 0, fmt.Errorf("file upload %q not found", target.FilePublicID)
	}

	objectKey, err := storageKeyFromFilestoreURI(fileUpload.StorageURI)
	if err != nil {
		return nil, "", 0, fmt.Errorf("parse storage path: %w", err)
	}

	st := filestore.GetStorage()
	obj, err := st.GetObject(ctx, filestore.DefaultBucket(), objectKey)
	if err != nil {
		return nil, "", 0, fmt.Errorf("read file from storage: %w", err)
	}

	return obj.Body, fileUpload.MimeType, fileUpload.FileSize, nil
}

func generateProjectPublicID() string {
	return fmt.Sprintf("prj_%s", snowflake.GenerateIDBase58())
}

func (s *projectService) buildRepoName(orgID uint, projectPublicID string) string {
	return fmt.Sprintf("%s-%d-%s", s.env, orgID, projectPublicID)
}

var visibleFolders = []string{"artifacts/", "uploads/"}

var ignoredFiles = map[string]bool{".gitkeep": true}

func isPathAllowed(filePath string) bool {
	name := filepath.Base(filePath)
	if ignoredFiles[name] {
		return false
	}
	for _, prefix := range visibleFolders {
		if strings.HasPrefix(filePath, prefix) {
			return true
		}
	}
	return false
}

// lookupFileCreatedAt 已移除，创建时间现在直接使用 ProjectFile.CreatedAt。
// 此文件中的一切 Gitea API 调用仅用于 Gitea 启用时的仓库初始化和 commit 记录查询。

func mimeTypeByExt(filename string) string {
	ext := filepath.Ext(filename)
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return ""
}

// buildFileTreeFromProjectFiles 将扁平的 ProjectFile 列表转换为 FileTreeNode 树结构
func buildFileTreeFromProjectFiles(ctx context.Context, dbParam *gorm.DB, files []types.ProjectFile) []*contract.FileTreeNode {
	var roots []*contract.FileTreeNode

	for _, pf := range files {
		fileUpload, err := db.GetFileUploadByPublicID(ctx, dbParam, pf.OrgID, pf.FilePublicID)
		if err != nil || fileUpload == nil {
			continue
		}

		var sourcePrefix string
		var fileName string
		if pf.ResourceType == types.ProjectFileResourceTypeArtifact {
			sourcePrefix = "artifacts/"
			fileName = fileUpload.OriginalName
		} else {
			sourcePrefix = "uploads/"
			fileName = fileUpload.OriginalName
		}
		fullPath := sourcePrefix + fileName

		node := &contract.FileTreeNode{
			Name:       fileName,
			Path:       fullPath,
			Type:       "file",
			Size:       fileUpload.FileSize,
			MimeType:   fileUpload.MimeType,
			CreatedAt:  pf.CreatedAt.Unix(),
			PublicID:   pf.FilePublicID,
			StorageURI: fileUpload.StorageURI,
			Sha256:     fileUpload.Sha256,
		}
		roots = append(roots, node)
	}

	return roots
}

func storageKeyFromFilestoreURI(uri string) (string, error) {
	_, _, key, err := storage.ParseURI(uri)
	if err != nil {
		return "", fmt.Errorf("parse storage uri: %w", err)
	}
	return key, nil
}

func removeSkillFromProjectMetadata(meta types.ObjectMetadata, skillName string) (types.ObjectMetadata, bool) {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" || meta.Extra == nil {
		return meta, false
	}

	rawSkills, ok := meta.Extra["skills"]
	if !ok || rawSkills == nil {
		return meta, false
	}

	skillsSlice, ok := rawSkills.([]interface{})
	if !ok {
		return meta, false
	}

	filtered := make([]interface{}, 0, len(skillsSlice))
	removed := false
	for _, item := range skillsSlice {
		if projectSkillEntryMatches(item, skillName) {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return meta, false
	}

	newExtra := make(map[string]interface{}, len(meta.Extra))
	for key, value := range meta.Extra {
		newExtra[key] = value
	}
	newExtra["skills"] = filtered

	newMeta := meta
	newMeta.Extra = newExtra
	return newMeta, true
}

func projectSkillEntryMatches(item interface{}, skillName string) bool {
	entry, ok := item.(map[string]interface{})
	if !ok {
		return false
	}
	code, _ := entry["code"].(string)
	name, _ := entry["name"].(string)
	target := strings.TrimSpace(skillName)
	return strings.EqualFold(strings.TrimSpace(code), target) ||
		strings.EqualFold(strings.TrimSpace(name), target)
}

func cleanupOrgProjectSkillReferences(ctx context.Context, database *gorm.DB, orgID uint, skillName string) (int, error) {
	projects, err := db.ListProjectsReferencingSkill(ctx, database, orgID, skillName)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, project := range projects {
		if project == nil {
			continue
		}
		newMeta, changed := removeSkillFromProjectMetadata(project.Metadata, skillName)
		if !changed {
			continue
		}
		project.Metadata = newMeta
		if err := db.UpdateProject(ctx, database, project); err != nil {
			logs.WarnContextf(ctx, "remove skill %q from project %s metadata: %v", skillName, project.PublicID, err)
			continue
		}
		updated++
	}
	return updated, nil
}

// ensure project implements contract.ProjectService at compile time
var _ contract.ProjectService = (*projectService)(nil)
