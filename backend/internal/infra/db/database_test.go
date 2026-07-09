package db

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/types"
)

func TestRunMigrationsCreatesOrganizationTables(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := runMigrations(database); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	for _, tableName := range []string{
		types.TableNameDepartment,
		types.TableNameMemberDepartment,
	} {
		if !database.Migrator().HasTable(tableName) {
			t.Fatalf("expected table %s to be migrated", tableName)
		}
	}
}

func setupVerifyProjectResourceBindingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&types.Project{}, &types.Resource{}, &types.ResourceBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func TestVerifyProjectResourceBindingsReturnsNilOnCompleteFixture(t *testing.T) {
	database := setupVerifyProjectResourceBindingsTestDB(t)

	ctx := context.Background()
	project := &types.Project{PublicID: "prj_verify_ok", OrgID: 1, OwnerID: 10, Name: "OK", Status: string(types.ProjectStatusActive)}
	if err := CreateProject(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	resource := &types.Resource{OrgID: 1, Uin: 10, Type: types.ResourceTypeProject, BizID: project.ID}
	if err := CreateResource(ctx, database, resource); err != nil {
		t.Fatalf("create resource: %v", err)
	}
	ownerUin := uint(10)
	if err := CreateResourceBinding(ctx, database, &types.ResourceBinding{
		OrgID: 1, Uin: &ownerUin, ResourceID: resource.ID, Role: types.ResourceRoleOwner,
	}); err != nil {
		t.Fatalf("create owner binding: %v", err)
	}

	if err := verifyProjectResourceBindings(database); err != nil {
		t.Fatalf("verifyProjectResourceBindings: %v", err)
	}
}

func TestVerifyProjectResourceBindingsReturnsNilWhenProjectMissingOwnerBinding(t *testing.T) {
	database := setupVerifyProjectResourceBindingsTestDB(t)

	ctx := context.Background()
	project := &types.Project{PublicID: "prj_verify_gap", OrgID: 1, OwnerID: 10, Name: "Gap", Status: string(types.ProjectStatusActive)}
	if err := CreateProject(ctx, database, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	resource := &types.Resource{OrgID: 1, Uin: 10, Type: types.ResourceTypeProject, BizID: project.ID}
	if err := CreateResource(ctx, database, resource); err != nil {
		t.Fatalf("create resource: %v", err)
	}

	if err := verifyProjectResourceBindings(database); err != nil {
		t.Fatalf("verifyProjectResourceBindings should warn-only, got err: %v", err)
	}
}
