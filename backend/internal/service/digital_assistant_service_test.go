package service

import (
	"context"
	"testing"

	"github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDigitalAssistantDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&types.DigitalAssistant{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return db
}

func setupDigitalAssistantProvisioningDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&types.DigitalAssistant{}, &types.WorkerDeployment{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return db
}

func TestCreateDigitalAssistant_ValidInput(t *testing.T) {
	db := setupDigitalAssistantDB(t)
	ctx := setupTestContextWithCaller(t)

	service := NewDigitalAssistantService(db, nil)

	req := &contract.CreateDigitalAssistantRequest{
		Name:         "Test Name",
		RoleName:     "测试专员",
		Description:  "Test Description",
		SystemPrompt: "You are a test assistant",
	}

	result, err := service.CreateDigitalAssistant(ctx, req)
	if err != nil {
		t.Fatalf("CreateDigitalAssistant failed: %v", err)
	}

	if result.PublicID == "" {
		t.Fatal("expected generated public_id")
	}
	if result.Name != "Test Name" {
		t.Errorf("expected name 'Test Name', got %s", result.Name)
	}
	if result.RoleName != "测试专员" {
		t.Errorf("expected role_name '测试专员', got %s", result.RoleName)
	}
}

func TestCreateDigitalAssistant_WithoutCaller(t *testing.T) {
	db := setupDigitalAssistantDB(t)
	ctx := setupTestContextWithoutCaller(t)

	service := NewDigitalAssistantService(db, nil)

	req := &contract.CreateDigitalAssistantRequest{
		Name: "Test Name",
	}

	_, err := service.CreateDigitalAssistant(ctx, req)
	if err == nil {
		t.Fatal("expected error when caller is not in context")
	}
	if err.Error() != "user not authenticated or org not set" {
		t.Errorf("expected 'user not authenticated or org not set', got %v", err)
	}
}

func TestCreateDigitalAssistant_GeneratesCodeWhenMissing(t *testing.T) {
	db := setupDigitalAssistantDB(t)
	ctx := setupTestContextWithCaller(t)

	service := NewDigitalAssistantService(db, nil)

	req := &contract.CreateDigitalAssistantRequest{
		Name: "Test Name",
	}

	result, err := service.CreateDigitalAssistant(ctx, req)
	if err != nil {
		t.Fatalf("CreateDigitalAssistant failed: %v", err)
	}
	if result.PublicID == "" {
		t.Fatal("expected generated public_id")
	}
}

func TestCreateDigitalAssistant_MissingName(t *testing.T) {
	db := setupDigitalAssistantDB(t)
	ctx := setupTestContextWithCaller(t)

	service := NewDigitalAssistantService(db, nil)

	req := &contract.CreateDigitalAssistantRequest{}

	_, err := service.CreateDigitalAssistant(ctx, req)
	if err == nil {
		t.Fatal("expected error when name is missing")
	}
}

func TestCreateDigitalAssistant_AllowsAllPresetRoles(t *testing.T) {
	db := setupDigitalAssistantDB(t)
	ctx := setupTestContextWithCaller(t)

	service := NewDigitalAssistantService(db, nil)
	for i := 0; i < 7; i++ {
		_, err := service.CreateDigitalAssistant(ctx, &contract.CreateDigitalAssistantRequest{
			Name: "Preset Role",
		})
		if err != nil {
			t.Fatalf("CreateDigitalAssistant #%d failed: %v", i+1, err)
		}
	}
}

func TestListDigitalAssistantExcludesSystemDefaultAssistant(t *testing.T) {
	database := setupDigitalAssistantDB(t)
	ctx := setupTestContextWithCaller(t)
	service := NewDigitalAssistantService(database, nil)

	assistants := []*types.DigitalAssistant{
		{PublicID: "assistant_default_o1", OrgID: 1, OwnerID: 1, Name: "System Default", Status: string(contract.DigitalAssistantStatusActive)},
		{PublicID: "assistant_custom_1", OrgID: 1, OwnerID: 1, Name: "Custom Assistant", Status: string(contract.DigitalAssistantStatusActive)},
	}
	if err := database.Create(&assistants).Error; err != nil {
		t.Fatalf("create assistants: %v", err)
	}

	result, err := service.ListDigitalAssistant(ctx, &contract.ListDigitalAssistantRequest{})
	if err != nil {
		t.Fatalf("ListDigitalAssistant failed: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("visible assistants = %d/%d, want 1/1", result.Total, len(result.Items))
	}
	if result.Items[0].PublicID != "assistant_custom_1" {
		t.Fatalf("public_id = %q, want assistant_custom_1", result.Items[0].PublicID)
	}
}

func TestUpdateDigitalAssistantRejectsTemplateCreatedAssistant(t *testing.T) {
	database := setupDigitalAssistantDB(t)
	ctx := setupTestContextWithCaller(t)
	service := NewDigitalAssistantService(database, nil)

	assistant := &types.DigitalAssistant{
		PublicID:     "template-assistant",
		OrgID:        1,
		OwnerID:      1,
		Name:         "Template Assistant",
		RoleName:     "投标策略师",
		Description:  "Original description",
		Avatar:       "file_original",
		SystemPrompt: "Template system prompt",
		Status:       string(contract.DigitalAssistantStatusActive),
		Source:       "template",
	}
	if err := database.Create(assistant).Error; err != nil {
		t.Fatalf("create template assistant: %v", err)
	}

	updated, err := service.UpdateDigitalAssistant(ctx, assistant.ID, &contract.UpdateDigitalAssistantRequest{
		Name:        "Modified Template Assistant",
		Description: "Modified description",
		Avatar:      "file_updated",
	})
	if err != nil {
		t.Fatalf("template user-field update failed: %v", err)
	}
	if updated.Name != "Modified Template Assistant" || updated.Description != "Modified description" || updated.Avatar != "file_updated" {
		t.Fatalf("template user fields were not updated: %+v", updated)
	}

	var stored types.DigitalAssistant
	if err := database.First(&stored, assistant.ID).Error; err != nil {
		t.Fatalf("reload template assistant: %v", err)
	}
	if stored.RoleName != "投标策略师" || stored.SystemPrompt != "Template system prompt" {
		t.Fatalf("template configuration changed: %+v", stored)
	}

	_, err = service.UpdateDigitalAssistant(ctx, assistant.ID, &contract.UpdateDigitalAssistantRequest{
		RoleName: "自定义角色",
	})
	if err == nil || err.Error() != "template-created digital assistant role name cannot be modified" {
		t.Fatalf("template role update error = %v", err)
	}
	customSystemPrompt := "自定义角色设定"
	_, err = service.UpdateDigitalAssistant(ctx, assistant.ID, &contract.UpdateDigitalAssistantRequest{
		SystemPrompt: &customSystemPrompt,
	})
	if err == nil || err.Error() != "template-created digital assistant system prompt cannot be modified" {
		t.Fatalf("template system prompt update error = %v", err)
	}
}

func TestOrganizationMemberCanUpdateAndDeleteSharedAssistant(t *testing.T) {
	database := setupDigitalAssistantDB(t)
	service := NewDigitalAssistantService(database, nil)
	assistant := &types.DigitalAssistant{
		PublicID: "shared-assistant",
		OrgID:    1,
		OwnerID:  1,
		Name:     "小法",
		RoleName: "法务专员",
		Status:   string(contract.DigitalAssistantStatusActive),
		Source:   "custom",
	}
	if err := database.Create(assistant).Error; err != nil {
		t.Fatalf("create shared assistant: %v", err)
	}

	memberCtx := auth.WithContext(context.Background(), &types.Caller{
		Uin:   2,
		OrgID: 1,
		Kind:  types.CallerKindUser,
		State: types.AuthStateSucc,
	}, nil)
	if _, err := service.UpdateDigitalAssistant(memberCtx, assistant.ID, &contract.UpdateDigitalAssistantRequest{
		Name: "法务小周",
	}); err != nil {
		t.Fatalf("organization member update failed: %v", err)
	}
	if err := service.DeleteDigitalAssistant(memberCtx, assistant.ID); err != nil {
		t.Fatalf("organization member delete failed: %v", err)
	}
}

func TestListDigitalAssistantReturnsCurrentOrganizationAssistants(t *testing.T) {
	database := setupDigitalAssistantDB(t)
	service := NewDigitalAssistantService(database, nil)
	assistants := []*types.DigitalAssistant{
		{PublicID: "org-one-owner-one", OrgID: 1, OwnerID: 1, Name: "小投", Status: "active"},
		{PublicID: "org-one-owner-two", OrgID: 1, OwnerID: 2, Name: "小法", Status: "active"},
		{PublicID: "org-two-owner-three", OrgID: 2, OwnerID: 3, Name: "小数", Status: "active"},
	}
	if err := database.Create(assistants).Error; err != nil {
		t.Fatalf("create assistants: %v", err)
	}

	memberCtx := auth.WithContext(context.Background(), &types.Caller{
		Uin:   2,
		OrgID: 1,
		Kind:  types.CallerKindUser,
		State: types.AuthStateSucc,
	}, nil)
	result, err := service.ListDigitalAssistant(memberCtx, &contract.ListDigitalAssistantRequest{
		Pagination: types.Pagination{ListAll: true},
	})
	if err != nil {
		t.Fatalf("ListDigitalAssistant failed: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("organization assistants = %d/%d, want 2/2", result.Total, len(result.Items))
	}
	for _, item := range result.Items {
		if item.OrgID != 1 {
			t.Fatalf("returned cross-organization assistant: %#v", item)
		}
	}
}

func TestListDigitalAssistantRejectsCallerWithoutOrganization(t *testing.T) {
	database := setupDigitalAssistantDB(t)
	service := NewDigitalAssistantService(database, nil)
	ctx := auth.WithContext(context.Background(), &types.Caller{
		Uin:   2,
		Kind:  types.CallerKindUser,
		State: types.AuthStateSucc,
	}, nil)

	if _, err := service.ListDigitalAssistant(ctx, &contract.ListDigitalAssistantRequest{}); err == nil {
		t.Fatal("expected caller without organization to be rejected")
	}
}

func TestOtherOrganizationCannotAccessSharedAssistant(t *testing.T) {
	database := setupDigitalAssistantDB(t)
	service := NewDigitalAssistantService(database, nil)
	assistant := &types.DigitalAssistant{
		PublicID: "org-one-assistant",
		OrgID:    1,
		OwnerID:  1,
		Name:     "小投",
		RoleName: "投标经理",
		Status:   string(contract.DigitalAssistantStatusActive),
		Source:   "custom",
	}
	if err := database.Create(assistant).Error; err != nil {
		t.Fatalf("create assistant: %v", err)
	}

	otherOrgCtx := auth.WithContext(context.Background(), &types.Caller{
		Uin:   2,
		OrgID: 2,
		Kind:  types.CallerKindUser,
		State: types.AuthStateSucc,
	}, nil)
	if _, err := service.GetDigitalAssistantByID(otherOrgCtx, assistant.ID); err == nil {
		t.Fatal("expected cross-organization access to be rejected")
	}
}

func TestUpdateDigitalAssistantStatusActiveMarksDeploymentPending(t *testing.T) {
	db := setupDigitalAssistantProvisioningDB(t)
	ctx := setupTestContextWithCaller(t)

	provisioning := NewWorkerProvisioningService(db, nil)
	service := NewDigitalAssistantServiceWithProvisioning(db, nil, provisioning)

	result, err := service.CreateDigitalAssistant(ctx, &contract.CreateDigitalAssistantRequest{
		Name:         "Deploy Pending",
		Description:  "wait for scheduler health",
		SystemPrompt: "stay pending until worker is ready",
	})
	if err != nil {
		t.Fatalf("CreateDigitalAssistant failed: %v", err)
	}

	if err := service.UpdateDigitalAssistantStatus(ctx, result.ID, &contract.UpdateDigitalAssistantStatusRequest{
		Status: string(contract.DigitalAssistantStatusActive),
	}); err != nil {
		t.Fatalf("UpdateDigitalAssistantStatus failed: %v", err)
	}

	var deployment types.WorkerDeployment
	if err := db.Where("digital_assistant_id = ?", result.ID).First(&deployment).Error; err != nil {
		t.Fatalf("reload deployment: %v", err)
	}
	if deployment.Status != string(types.WorkerDeploymentStatusPending) {
		t.Fatalf("deployment status = %q, want pending", deployment.Status)
	}
	if deployment.PublicID == "" {
		t.Fatal("deployment public_id is empty")
	}

	updated, err := service.GetDigitalAssistantByID(ctx, result.ID)
	if err != nil {
		t.Fatalf("GetDigitalAssistantByID failed: %v", err)
	}
	if updated.Deployment == nil {
		t.Fatal("deployment is nil")
	}
	if updated.Deployment.PublicID != deployment.PublicID {
		t.Fatalf("deployment public_id = %q, want %q", updated.Deployment.PublicID, deployment.PublicID)
	}
}
