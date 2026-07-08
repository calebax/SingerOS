package service

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	localauth "github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

func TestOrgServiceCreateOrgEnsuresDefaultWorker(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&types.Organization{}, &types.Department{}, &types.DigitalAssistant{}, &types.WorkerDeployment{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	ctx := context.Background()
	authCtx := localauth.WithContext(ctx, &types.Caller{
		Uin:   42,
		Kind:  types.CallerKindUser,
		State: types.AuthStateSucc,
	}, &types.Trace{})
	service := NewOrgServiceWithProvisioning(database, NewWorkerProvisioningService(database, nil))

	created, err := service.CreateOrg(authCtx, &contract.CreateOrgRequest{
		Name: "新工作组织",
		Code: "new-work-org",
	})
	if err != nil {
		t.Fatalf("CreateOrg failed: %v", err)
	}
	if created.PublicID == "" || created.Code != "new-work-org" {
		t.Fatalf("unexpected created org: %#v", created)
	}

	org, err := db.GetOrgByCode(ctx, database, "new-work-org")
	if err != nil {
		t.Fatalf("GetOrgByCode failed: %v", err)
	}
	if org == nil {
		t.Fatal("expected organization")
	}

	assistant, err := db.GetDigitalAssistantByCode(ctx, database, defaultWorkerCode(org.ID))
	if err != nil {
		t.Fatalf("GetDigitalAssistantByCode failed: %v", err)
	}
	if assistant == nil {
		t.Fatal("expected default assistant")
	}
	if assistant.OwnerID != 42 {
		t.Fatalf("default assistant owner_id = %d, want 42", assistant.OwnerID)
	}

	deployment, err := db.GetDefaultWorkerDeployment(ctx, database, org.ID)
	if err != nil {
		t.Fatalf("GetDefaultWorkerDeployment failed: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected default worker deployment")
	}
	if deployment.DigitalAssistantID != assistant.ID || deployment.WorkerID != 1 {
		t.Fatalf("unexpected default worker deployment: %#v", deployment)
	}
	if deployment.Status != string(types.WorkerDeploymentStatusPending) {
		t.Fatalf("deployment status = %q, want pending", deployment.Status)
	}
}
