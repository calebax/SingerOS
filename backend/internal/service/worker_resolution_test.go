package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

// setupResolveTestDB 为 resolveProjectAssistantWorker 测试构造内存 sqlite 库：
// seed project resource + assistant bindings（权限来源）。
func setupResolveTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	d, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := d.AutoMigrate(
		&types.Project{},
		&types.ProjectMember{},
		&types.WorkerDeployment{},
		&types.Resource{},
		&types.ResourceBinding{},
		&types.DigitalAssistant{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	_ = infradb.CreateProject(ctx, d, &types.Project{PublicID: "p1", OrgID: 1, OwnerID: 1, Name: "P1", Status: string(types.ProjectStatusActive)})
	for _, assistantID := range []uint{100, 200} {
		if err := d.Exec(fmt.Sprintf(
			`INSERT INTO %s (id, created_at, updated_at, public_id, org_id, owner_id, name, status, system_prompt, source) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, 1, 1, ?, 'active', 'test', 'custom')`,
			types.TableNameDigitalAssistant,
		), assistantID, fmt.Sprintf("assistant-%d", assistantID), fmt.Sprintf("Assistant %d", assistantID)).Error; err != nil {
			t.Fatalf("create assistant %d: %v", assistantID, err)
		}
	}
	resource := &types.Resource{OrgID: 1, Uin: 1, Type: types.ResourceTypeProject, BizID: 1}
	if err := infradb.CreateResource(ctx, d, resource); err != nil {
		t.Fatalf("create resource: %v", err)
	}
	seedAssistantBinding(t, ctx, d, resource.ID, 100)
	seedAssistantBinding(t, ctx, d, resource.ID, 200)
	_ = infradb.CreateWorkerDeployment(ctx, d, &types.WorkerDeployment{PublicID: "dep-100", OrgID: 1, DigitalAssistantID: 100, WorkerID: 1000, DeploymentName: "dep-100", Status: string(types.WorkerDeploymentStatusReady)})
	_ = infradb.CreateWorkerDeployment(ctx, d, &types.WorkerDeployment{PublicID: "dep-200", OrgID: 1, DigitalAssistantID: 200, WorkerID: 2000, DeploymentName: "dep-200", Status: string(types.WorkerDeploymentStatusReady)})
	_ = infradb.CreateProject(ctx, d, &types.Project{PublicID: "p2", OrgID: 1, OwnerID: 1, Name: "P2", Status: string(types.ProjectStatusActive)})
	return d
}

func seedAssistantBinding(t *testing.T, ctx context.Context, d *gorm.DB, resourceID, assistantID uint) {
	t.Helper()
	id := assistantID
	if err := infradb.CreateResourceBinding(ctx, d, &types.ResourceBinding{
		OrgID:       1,
		AssistantID: &id,
		ResourceID:  resourceID,
		Role:        types.ResourceRoleMember,
	}); err != nil {
		t.Fatalf("seed assistant binding %d: %v", assistantID, err)
	}
}

// 未传 assistantIDs + 项目有 assistant binding → 返回最新绑定的助手 (200)。
func TestResolveProjectAssistantWorkerPicksLatestByDefault(t *testing.T) {
	d := setupResolveTestDB(t)
	got, _, err := resolveProjectAssistantWorker(context.Background(), d, 1, 1, nil, nil)
	if err != nil || got != 200 {
		t.Fatalf("want 200, got %d err %v", got, err)
	}
}

// 未传 assistantIDs + 项目无 assistant binding → ErrNoDefaultAssistant。
func TestResolveProjectAssistantWorkerErrorsWhenNoAssistant(t *testing.T) {
	d := setupResolveTestDB(t)
	_, _, err := resolveProjectAssistantWorker(context.Background(), d, 1, 2, nil, nil)
	if !errors.Is(err, ErrNoDefaultAssistant) {
		t.Fatalf("want ErrNoDefaultAssistant, got %v", err)
	}
}

// 传 assistantIDs + 非项目 assistant binding → 错误。
func TestResolveProjectAssistantWorkerValidatesMembership(t *testing.T) {
	d := setupResolveTestDB(t)
	_, _, err := resolveProjectAssistantWorker(context.Background(), d, 1, 1, []uint{999}, nil)
	if err == nil {
		t.Fatal("non-member should be rejected")
	}
}

// 传 assistantIDs + 是项目 assistant binding → 返回该 worker（happy path）。
func TestResolveProjectAssistantWorkerReturnsWorkerForValidMember(t *testing.T) {
	d := setupResolveTestDB(t)
	assistantID, workerID, err := resolveProjectAssistantWorker(context.Background(), d, 1, 1, []uint{100}, nil)
	if err != nil {
		t.Fatalf("valid member should resolve: %v", err)
	}
	if assistantID != 100 {
		t.Fatalf("assistantID = %d, want 100", assistantID)
	}
	if workerID != 1000 {
		t.Fatalf("workerID = %d, want 1000", workerID)
	}
}
