package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
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

	if err := db.CreateProject(ctx, s.db, project); err != nil {
		return nil, err
	}
	if project.GiteaRepoFullName != "" {
		if err := git.InitRepoStructure(ctx, s.giteaClient, project.GiteaRepoFullName); err != nil {
			logs.WarnContextf(ctx, "[project] init repo structure: %v", err)
		}
	}

	if err := s.bindProjectMembers(ctx, project.ID, caller, req.Members); err != nil {
		return nil, err
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

		if err := db.UpdateProject(ctx, tx, project); err != nil {
			return err
		}

		if len(req.Members) > 0 {
			if err := s.syncProjectMembers(ctx, tx, project.ID, caller.OrgID, caller.Uin, req.Members); err != nil {
				return err
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
func (s *projectService) bindProjectMembers(ctx context.Context, projectID uint, caller *types.Caller, inputs []contract.MemberInput) error {
	defaultAssistantID, err := db.GetDefaultAssistantIDByOrg(ctx, s.db, caller.OrgID)
	if err != nil {
		return fmt.Errorf("get default assistant: %w", err)
	}
	if defaultAssistantID == 0 {
		return ErrNoDefaultAssistantInOrg
	}

	assistantPublicIDs, userPublicIDs, err := parseMemberInputs(inputs)
	if err != nil {
		return err
	}

	assistantIDs, err := resolveAssistantIDsByPublicID(ctx, s.db, caller.OrgID, assistantPublicIDs)
	if err != nil {
		return fmt.Errorf("resolve assistant public ids: %w", err)
	}

	if err := validateAssistantIDs(assistantIDs, defaultAssistantID); err != nil {
		return err
	}

	userUins, err := db.GetUinsByPublicIDs(ctx, s.db, caller.OrgID, userPublicIDs)
	if err != nil {
		return fmt.Errorf("get user uins: %w", err)
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

	return db.BatchCreateProjectMembers(ctx, s.db, members)
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
func (s *projectService) syncProjectMembers(ctx context.Context, tx *gorm.DB, projectID, orgID, callerUin uint, inputs []contract.MemberInput) error {
	defaultAssistantID, err := db.GetDefaultAssistantIDByOrg(ctx, tx, orgID)
	if err != nil {
		return fmt.Errorf("get default assistant: %w", err)
	}
	if defaultAssistantID == 0 {
		return ErrNoDefaultAssistantInOrg
	}

	assistantPublicIDs, userPublicIDs, err := parseMemberInputs(inputs)
	if err != nil {
		return err
	}

	assistantIDs, err := resolveAssistantIDsByPublicID(ctx, tx, orgID, assistantPublicIDs)
	if err != nil {
		return fmt.Errorf("resolve assistant public ids: %w", err)
	}

	if err := validateAssistantIDs(assistantIDs, defaultAssistantID); err != nil {
		return err
	}

	userUins, err := db.GetUinsByPublicIDs(ctx, tx, orgID, userPublicIDs)
	if err != nil {
		return fmt.Errorf("get user uins: %w", err)
	}

	now := time.Now()

	// 同步 AI 队友
	existingAssistants, err := db.ListProjectAssistantMembers(ctx, tx, projectID)
	if err != nil {
		return fmt.Errorf("list project assistants: %w", err)
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
	for _, m := range existingNonDefault {
		if !requestedAssistantSet[m.MemberID] {
			if err := db.DeleteProjectMember(ctx, tx, m.ID); err != nil {
				return fmt.Errorf("delete project assistant member %d: %w", m.MemberID, err)
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
				return fmt.Errorf("create project assistant member %d: %w", id, err)
			}
		}
	}

	// 同步用户成员
	existingUsers, err := db.ListProjectMemberByType(ctx, tx, projectID, types.MemberTypeUser)
	if err != nil {
		return fmt.Errorf("list project user members: %w", err)
	}
	existingUserMap := make(map[uint]*types.ProjectMember)
	for _, m := range existingUsers {
		if m.MemberRole == types.MemberRoleOwner {
			continue
		}
		existingUserMap[m.MemberID] = m
	}
	requestedUserSet := make(map[uint]bool, len(userUins))
	for _, uin := range userUins {
		requestedUserSet[uin] = true
	}
	for _, m := range existingUserMap {
		if !requestedUserSet[m.MemberID] {
			if err := db.DeleteProjectMember(ctx, tx, m.ID); err != nil {
				return fmt.Errorf("delete project user member %d: %w", m.MemberID, err)
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
				return fmt.Errorf("create project user member %d: %w", uin, err)
			}
		}
	}

	return nil
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
				// 中文注释：项目成员弹窗依赖 public_id 判断候选项是否已加入项目。
				item.PublicID = u.PublicID
				item.Name = u.Name
				item.AvatarURL = u.AvatarURL
			}
		} else if m.MemberType == types.MemberTypeAssistant {
			if a, ok := assistantMap[m.MemberID]; ok {
				// 中文注释：AI 队友同样返回 public_id，避免前端只能用内部 ID 匹配导致漏过滤。
				item.PublicID = a.PublicID
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
