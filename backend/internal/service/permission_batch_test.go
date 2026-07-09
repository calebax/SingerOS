package service

import (
	"context"
	"testing"

	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBatchCheckTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(
		&types.Project{},
		&types.Resource{},
		&types.ResourceBinding{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return database
}

func TestBatchCheckByPublicID_ProjectView(t *testing.T) {
	ctx := context.Background()
	gdb := setupBatchCheckTestDB(t)

	orgID := uint(91001)
	uin := uint(91002)
	projectID := uint(91003)
	publicID := "proj-batch-check"

	if err := gdb.Create(&types.Project{
		Model:    gorm.Model{ID: projectID},
		OrgID:    orgID,
		PublicID: publicID,
		Name:     "batch-check-project",
	}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	resource := &types.Resource{
		OrgID: orgID,
		Type:  types.ResourceTypeProject,
		BizID: projectID,
	}
	if err := db.CreateResource(ctx, gdb, resource); err != nil {
		t.Fatalf("create resource: %v", err)
	}
	binding := &types.ResourceBinding{
		OrgID:      orgID,
		Uin:        &uin,
		ResourceID: resource.ID,
		Role:       types.ResourceRoleMember,
	}
	if err := db.CreateResourceBinding(ctx, gdb, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	svc := NewPermissionService(gdb)
	results, err := svc.BatchCheckByPublicID(ctx, Caller{OrgID: orgID, Uin: uin}, []BatchCheckItem{
		{
			Action:       ActionProjectView,
			ResourceType: types.ResourceTypeProject,
			PublicID:     publicID,
		},
		{
			Action:       ActionProjectDelete,
			ResourceType: types.ResourceTypeProject,
			PublicID:     publicID,
		},
	})
	if err != nil {
		t.Fatalf("BatchCheckByPublicID: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Allowed {
		t.Fatalf("expected project:view allowed, got reason=%s", results[0].Reason)
	}
	if results[1].Allowed {
		t.Fatal("expected project:delete denied for member")
	}
}

func TestBatchCheckByPublicID_ProjectMemberLeave(t *testing.T) {
	ctx := context.Background()
	gdb := setupBatchCheckTestDB(t)

	orgID := uint(92001)
	uin := uint(92002)
	projectID := uint(92003)
	publicID := "proj-batch-leave"

	if err := gdb.Create(&types.Project{
		Model:    gorm.Model{ID: projectID},
		OrgID:    orgID,
		PublicID: publicID,
		Name:     "batch-leave-project",
	}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	resource := &types.Resource{
		OrgID: orgID,
		Type:  types.ResourceTypeProject,
		BizID: projectID,
	}
	if err := db.CreateResource(ctx, gdb, resource); err != nil {
		t.Fatalf("create resource: %v", err)
	}
	binding := &types.ResourceBinding{
		OrgID:      orgID,
		Uin:        &uin,
		ResourceID: resource.ID,
		Role:       types.ResourceRoleMember,
	}
	if err := db.CreateResourceBinding(ctx, gdb, binding); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	svc := NewPermissionService(gdb)
	results, err := svc.BatchCheckByPublicID(ctx, Caller{OrgID: orgID, Uin: uin}, []BatchCheckItem{
		{
			Action:       ActionProjectMemberLeave,
			ResourceType: types.ResourceTypeProject,
			PublicID:     publicID,
		},
	})
	if err != nil {
		t.Fatalf("BatchCheckByPublicID: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Allowed {
		t.Fatalf("expected project:member.leave allowed, got reason=%s", results[0].Reason)
	}
}
