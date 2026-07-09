package service

import (
	"context"
	"testing"

	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPermissionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(
		&types.Resource{},
		&types.ResourceBinding{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return database
}

// TestPermissionService_ProjectOwnerCanView 验证项目 owner 对自己项目有 view 权限。
func TestPermissionService_ProjectOwnerCanView(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	projectResource := &types.Resource{
		OrgID: 1,
		Type:  types.ResourceTypeProject,
		BizID: 1001,
	}
	if err := db.CreateResource(ctx, database, projectResource); err != nil {
		t.Fatalf("create resource: %v", err)
	}

	uid := uint(42)
	binding := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: projectResource.ID,
		Uin:        &uid,
		Role:       types.ResourceRoleOwner,
	}
	if err := db.CreateResourceBinding(ctx, database, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 42}
	decision, err := svc.Can(ctx, caller, ActionProjectView, ResourceRef{Type: types.ResourceTypeProject, BizID: 1001}, nil)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed, got denied with reason %q", decision.Reason)
	}
	if decision.Role != types.ResourceRoleOwner {
		t.Fatalf("expected role owner, got %q", decision.Role)
	}
	if decision.ResourceID != projectResource.ID {
		t.Fatalf("expected resource id %d, got %d", projectResource.ID, decision.ResourceID)
	}
}

// TestPermissionService_FileInheritsProjectRole 验证文件继承项目 owner 角色。
func TestPermissionService_FileInheritsProjectRole(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 2001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	uid := uint(7)
	ownerBinding := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &uid,
		Role:       types.ResourceRoleOwner,
	}
	if err := db.CreateResourceBinding(ctx, database, ownerBinding); err != nil {
		t.Fatalf("create owner binding: %v", err)
	}

	file := &types.Resource{
		OrgID:                 1,
		Type:                  types.ResourceTypeFile,
		BizID:                 2002,
		ParentResourceID:      &project.ID,
		ParentResourcePathIDs: types.ResourcePathIDs{project.ID},
	}
	if err := db.CreateResource(ctx, database, file); err != nil {
		t.Fatalf("create file: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 7}
	decision, err := svc.Can(ctx, caller, ActionFileView, ResourceRef{Type: types.ResourceTypeFile, BizID: 2002}, nil)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed via inheritance, got denied with reason %q", decision.Reason)
	}
	if decision.Role != types.ResourceRoleOwner {
		t.Fatalf("expected inherited role owner, got %q", decision.Role)
	}
	if decision.MatchedResourceID != project.ID {
		t.Fatalf("expected matched resource id %d, got %d", project.ID, decision.MatchedResourceID)
	}
}

// TestPermissionService_ArtifactInheritsProjectRole 验证产物继承项目 owner 角色。
func TestPermissionService_ArtifactInheritsProjectRole(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 2001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	uid := uint(7)
	ownerBinding := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &uid,
		Role:       types.ResourceRoleOwner,
	}
	if err := db.CreateResourceBinding(ctx, database, ownerBinding); err != nil {
		t.Fatalf("create owner binding: %v", err)
	}

	artifact := &types.Resource{
		OrgID:                 1,
		Type:                  types.ResourceTypeArtifact,
		BizID:                 2002,
		ParentResourceID:      &project.ID,
		ParentResourcePathIDs: types.ResourcePathIDs{project.ID},
	}
	if err := db.CreateResource(ctx, database, artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 7}
	decision, err := svc.Can(ctx, caller, ActionArtifactView, ResourceRef{Type: types.ResourceTypeArtifact, BizID: 2002}, nil)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed via inheritance, got denied with reason %q", decision.Reason)
	}
	if decision.Role != types.ResourceRoleOwner {
		t.Fatalf("expected inherited role owner, got %q", decision.Role)
	}
	if decision.MatchedResourceID != project.ID {
		t.Fatalf("expected matched resource id %d, got %d", project.ID, decision.MatchedResourceID)
	}
}

// TestPermissionService_TaskInheritsProjectRole 验证任务继承项目 owner 角色。
func TestPermissionService_TaskInheritsProjectRole(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 3001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	uid := uint(9)
	ownerBinding := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &uid,
		Role:       types.ResourceRoleOwner,
	}
	if err := db.CreateResourceBinding(ctx, database, ownerBinding); err != nil {
		t.Fatalf("create owner binding: %v", err)
	}

	taskResource := &types.Resource{
		OrgID:                 1,
		Type:                  types.ResourceTypeTask,
		BizID:                 4001,
		ParentResourceID:      &project.ID,
		ParentResourcePathIDs: types.ResourcePathIDs{project.ID},
	}
	if err := db.CreateResource(ctx, database, taskResource); err != nil {
		t.Fatalf("create task resource: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 9}
	decision, err := svc.Can(ctx, caller, ActionTaskView, ResourceRef{Type: types.ResourceTypeTask, BizID: 4001}, nil)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed via inheritance, got denied with reason %q", decision.Reason)
	}
	if decision.Role != types.ResourceRoleOwner {
		t.Fatalf("expected inherited role owner, got %q", decision.Role)
	}
}

// TestPermissionService_NoBindingDenied 验证无绑定时默认拒绝。
func TestPermissionService_NoBindingDenied(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 3001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 99}
	decision, err := svc.Can(ctx, caller, ActionProjectView, ResourceRef{Type: types.ResourceTypeProject, BizID: 3001}, nil)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied without binding")
	}
	if decision.Reason != reasonNoBinding {
		t.Fatalf("expected reason no_binding, got %q", decision.Reason)
	}
}

// TestPermissionService_OrgMismatchDenied 验证组织隔离。
// 由于 DAO 按 org_id 过滤，跨组织资源对 caller 不可见，因此返回 resource_not_found。
func TestPermissionService_OrgMismatchDenied(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 2, Type: types.ResourceTypeProject, BizID: 4001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 1}
	decision, err := svc.Can(ctx, caller, ActionProjectView, ResourceRef{Type: types.ResourceTypeProject, BizID: 4001}, nil)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected denied due to org mismatch")
	}
	// 跨组织资源按 org_id 查不到，统一视为资源不存在。
	if decision.Reason != reasonResourceNotFound {
		t.Fatalf("expected reason resource_not_found, got %q", decision.Reason)
	}
}

// TestPermissionService_AdminCannotCreateOwner 验证 admin 不能创建 owner。
func TestPermissionService_AdminCannotCreateOwner(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 6001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	uid := uint(9)
	binding := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &uid,
		Role:       types.ResourceRoleAdmin,
	}
	if err := db.CreateResourceBinding(ctx, database, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 9}
	input := &MemberInput{TargetUin: 10, RequestedRole: types.ResourceRoleOwner}
	decision, err := svc.Can(ctx, caller, ActionProjectMemberCreate, ResourceRef{Type: types.ResourceTypeProject, BizID: 6001}, input)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected admin cannot create owner")
	}
}

// TestPermissionService_OwnerCannotDemoteLastOwner 验证不能降级项目最后一个 owner。
func TestPermissionService_OwnerCannotDemoteLastOwner(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 8001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	ownerUin := uint(11)
	coOwnerUin := uint(12)
	for _, uid := range []uint{ownerUin, coOwnerUin} {
		u := uid
		if err := db.CreateResourceBinding(ctx, database, &types.ResourceBinding{
			OrgID:      1,
			ResourceID: project.ID,
			Uin:        &u,
			Role:       types.ResourceRoleOwner,
		}); err != nil {
			t.Fatalf("create owner binding: %v", err)
		}
	}

	// 移除 co-owner 后只剩 ownerUin 一个 owner
	if err := database.Where("uin = ?", coOwnerUin).Delete(&types.ResourceBinding{}).Error; err != nil {
		t.Fatalf("delete co-owner binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: ownerUin}
	input := &MemberInput{TargetUin: ownerUin, RequestedRole: types.ResourceRoleMember}
	decision, err := svc.Can(ctx, caller, ActionProjectMemberUpdate, ResourceRef{Type: types.ResourceTypeProject, BizID: 8001}, input)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected owner cannot demote last owner")
	}
	if decision.Reason != reasonMemberContextDenied {
		t.Fatalf("expected reason %q, got %q", reasonMemberContextDenied, decision.Reason)
	}
}

// TestPermissionService_MemberCanLeaveSelf 验证 member 可以自助退出项目。
func TestPermissionService_MemberCanLeaveSelf(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 8101}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	memberUin := uint(21)
	u := memberUin
	if err := db.CreateResourceBinding(ctx, database, &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &u,
		Role:       types.ResourceRoleMember,
	}); err != nil {
		t.Fatalf("create member binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: memberUin}
	input := &MemberInput{TargetUin: memberUin}
	decision, err := svc.Can(ctx, caller, ActionProjectMemberLeave, ResourceRef{Type: types.ResourceTypeProject, BizID: 8101}, input)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected member can leave self, reason=%s", decision.Reason)
	}
}

// TestPermissionService_LastOwnerCannotLeave 验证最后一个 owner 不能退出项目。
func TestPermissionService_LastOwnerCannotLeave(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 8102}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	ownerUin := uint(22)
	u := ownerUin
	if err := db.CreateResourceBinding(ctx, database, &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &u,
		Role:       types.ResourceRoleOwner,
	}); err != nil {
		t.Fatalf("create owner binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: ownerUin}
	input := &MemberInput{TargetUin: ownerUin}
	decision, err := svc.Can(ctx, caller, ActionProjectMemberLeave, ResourceRef{Type: types.ResourceTypeProject, BizID: 8102}, input)
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected last owner cannot leave")
	}
}

// TestGuardActions_MemberLeave 验证 HTTP PermGuard 路径对 member.leave 注入 self 上下文。
func TestGuardActions_MemberLeave(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 8103}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create resource: %v", err)
	}

	memberUin := uint(31)
	u := memberUin
	if err := db.CreateResourceBinding(ctx, database, &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &u,
		Role:       types.ResourceRoleMember,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: memberUin}
	if err := svc.guardActions(ctx, caller, ResourceRef{Type: types.ResourceTypeProject, BizID: 8103}, ActionProjectMemberLeave); err != nil {
		t.Fatalf("guardActions leave: %v", err)
	}
}

// TestPermissionService_BatchCan 验证批量判断。
func TestPermissionService_BatchCan(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 7001}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	uid := uint(10)
	binding := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: project.ID,
		Uin:        &uid,
		Role:       types.ResourceRoleOwner,
	}
	if err := db.CreateResourceBinding(ctx, database, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 10}
	actions := []Action{ActionProjectView, ActionProjectDelete}
	refs := []ResourceRef{
		{Type: types.ResourceTypeProject, BizID: 7001},
		{Type: types.ResourceTypeProject, BizID: 7001},
	}
	results, err := svc.BatchCan(ctx, caller, actions, refs, nil)
	if err != nil {
		t.Fatalf("BatchCan: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Allowed {
		t.Fatalf("expected view allowed, got %q", results[0].Reason)
	}
	if !results[1].Allowed {
		t.Fatalf("expected delete allowed for owner, got %q", results[1].Reason)
	}
}

// TestResourceBindingValidate_BothEmpty 验证 types.ResourceBinding.Validate() 拒绝两者均空。
func TestResourceBindingValidate_BothEmpty(t *testing.T) {
	b := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: 1,
		Uin:        nil,
		Role:       types.ResourceRoleOwner,
	}
	if err := b.Validate(); err != types.ErrBindingIdentityRequired {
		t.Fatalf("expected ErrBindingIdentityRequired, got %v", err)
	}
}

// TestResourceBindingValidate_InvalidRole 验证 types.ResourceBinding.Validate() 拒绝非法角色。
func TestResourceBindingValidate_InvalidRole(t *testing.T) {
	uinVal := uint(1)
	b := &types.ResourceBinding{
		OrgID:      1,
		ResourceID: 1,
		Uin:        &uinVal,
		Role:       "super_admin",
	}
	if err := b.Validate(); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

// TestBatchCan_SameResourceUsesSingleEvaluation 验证同资源多 action 批量判断。
func TestBatchCan_SameResourceUsesSingleEvaluation(t *testing.T) {
	database := setupPermissionTestDB(t)
	svc := NewPermissionService(database)
	ctx := context.Background()

	project := &types.Resource{OrgID: 1, Type: types.ResourceTypeProject, BizID: 9002}
	if err := db.CreateResource(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	uid := uint(11)
	if err := db.CreateResourceBinding(ctx, database, &types.ResourceBinding{
		OrgID: 1, ResourceID: project.ID, Uin: &uid, Role: types.ResourceRoleMember,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	caller := Caller{OrgID: 1, Uin: 11}
	actions := []Action{ActionProjectView, ActionProjectMemberList}
	refs := []ResourceRef{
		{Type: types.ResourceTypeProject, BizID: 9002},
		{Type: types.ResourceTypeProject, BizID: 9002},
	}
	results, err := svc.BatchCan(ctx, caller, actions, refs, nil)
	if err != nil {
		t.Fatalf("BatchCan: %v", err)
	}
	if len(results) != 2 || !results[0].Allowed || !results[1].Allowed {
		t.Fatalf("expected both allowed, got %#v", results)
	}
}
