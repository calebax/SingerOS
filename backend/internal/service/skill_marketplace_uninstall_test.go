package service

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/pkg/messaging"
	"github.com/insmtx/Leros/backend/types"
)

func expectDeleteOrgSkillInstallationByName(mock sqlmock.Sqlmock) {
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "leros_org_skill_installation" SET`)).
		WithArgs(sqlmock.AnyArg(), uint(100), "demo-skill", "demo-skill").
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectOrgWorkersForPublish(mock sqlmock.Sqlmock) {
	columns := []string{"id", "created_at", "updated_at", "deleted_at", "public_id",
		"org_id", "digital_assistant_id", "worker_id", "deployment_name", "namespace",
		"status", "bootstrap_token_hash", "workspace_path", "last_error",
		"last_started_at", "last_reconciled_at"}
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "leros_worker_deployment" WHERE org_id = $1 AND status IN ($2,$3) AND "leros_worker_deployment"."deleted_at" IS NULL ORDER BY worker_id ASC`)).
		WithArgs(uint(100), "ready", "provisioning").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			1, now, now, nil, "wrk_test_default",
			100, 200, 1, "test-worker", "",
			string(types.WorkerDeploymentStatusReady), "", "", "", nil, nil,
		))
}

func expectProjectsReferencingSkill(mock sqlmock.Sqlmock, skillName string) {
	columns := []string{
		"id", "created_at", "updated_at", "deleted_at", "public_id",
		"org_id", "owner_id", "name", "description", "objective", "status",
		"gitea_repo_full_name", "gitea_repo_id", "gitea_default_branch", "metadata",
	}
	now := time.Now()
	metadata := []byte(`{"extra":{"skills":[{"code":"demo-skill","name":"Demo Skill"}]}}`)
	mock.ExpectQuery(`SELECT .* FROM "leros_project" WHERE \(org_id = \$1 AND deleted_at IS NULL\) AND \(EXISTS`).
		WithArgs(uint(100), skillName, skillName).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			1, now, now, nil, "prj_demo",
			100, 1, "Demo Project", "", "", "active",
			"", 0, "main", metadata,
		))
}

func TestUninstallSkillCleansProjectReferencesAfterWorkerSuccess(t *testing.T) {
	database, mock, ctx, cleanup := setupSkillMarketplaceInstallServiceDB(t)
	defer cleanup()
	expectDefaultWorkerDeployment(mock)
	expectDeleteOrgSkillInstallationByName(mock)
	expectOrgWorkersForPublish(mock)
	expectProjectsReferencingSkill(mock, "demo-skill")
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "leros_project" SET`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	publisher := &skillInstallPublisher{
		response: messaging.WorkerCommandResult{
			Success: true,
			Action:  "uninstall",
			Message: "skill \"demo-skill\" uninstalled",
		},
	}
	service := NewSkillMarketplaceService(database, publisher, nil, "")

	resp, err := service.UninstallSkill(ctx, &contract.UninstallSkillRequest{Name: "demo-skill"})
	if err != nil {
		t.Fatalf("uninstall skill: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("status = %q, want accepted", resp.Status)
	}
	if !strings.Contains(resp.Message, "removed from 1 project(s)") {
		t.Fatalf("message = %q, want project cleanup note", resp.Message)
	}
	if len(publisher.requests) < 1 {
		t.Fatalf("request count = %d, want at least 1", len(publisher.requests))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUninstallSkillWorkerFailureDoesNotCleanProjectReferences(t *testing.T) {
	database, mock, ctx, cleanup := setupSkillMarketplaceInstallServiceDB(t)
	defer cleanup()
	expectDefaultWorkerDeployment(mock)
	expectDeleteOrgSkillInstallationByName(mock)
	expectOrgWorkersForPublish(mock)
	publisher := &skillInstallPublisher{
		err: errors.New("worker unavailable"),
	}
	service := NewSkillMarketplaceService(database, publisher, nil, "")

	resp, err := service.UninstallSkill(ctx, &contract.UninstallSkillRequest{Name: "demo-skill"})
	if err != nil {
		t.Fatalf("uninstall skill: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("status = %q, want accepted", resp.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUninstallSkillUsesRequestReply(t *testing.T) {
	database, mock, ctx, cleanup := setupSkillMarketplaceInstallServiceDB(t)
	defer cleanup()
	expectDefaultWorkerDeployment(mock)
	expectDeleteOrgSkillInstallationByName(mock)
	expectOrgWorkersForPublish(mock)
	mock.ExpectQuery(`SELECT .* FROM "leros_project" WHERE \(org_id = \$1 AND deleted_at IS NULL\) AND \(EXISTS`).
		WithArgs(uint(100), "demo-skill", "demo-skill").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	publisher := &skillInstallPublisher{
		response: messaging.WorkerCommandResult{
			Success: true,
			Action:  "uninstall",
			Message: "skill \"demo-skill\" uninstalled",
		},
	}
	service := NewSkillMarketplaceService(database, publisher, nil, "")

	_, err := service.UninstallSkill(ctx, &contract.UninstallSkillRequest{Name: "demo-skill"})
	if err != nil {
		t.Fatalf("uninstall skill: %v", err)
	}
	if len(publisher.requests) < 1 {
		t.Fatalf("request count = %d, want at least 1", len(publisher.requests))
	}
	found := false
	for _, req := range publisher.requests {
		wcmd, ok := req.(messaging.WorkerCommand)
		if !ok {
			continue
		}
		payload, err := messaging.DecodeCommandPayload[messaging.SkillCommandPayload](&wcmd.Body)
		if err != nil {
			continue
		}
		if payload.Action == "uninstall" && payload.Name == "demo-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a WorkerCommand with uninstall action for demo-skill, got %d requests", len(publisher.requests))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
