package service

import (
	"context"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

func TestRemoveSkillFromProjectMetadata_MatchesCode(t *testing.T) {
	meta := types.ObjectMetadata{
		Tags: []string{"tag-a"},
		Type: "demo",
		Extra: map[string]interface{}{
			"skills": []interface{}{
				map[string]interface{}{"code": "demo-skill", "name": "Demo Skill"},
				map[string]interface{}{"code": "other-skill", "name": "Other"},
			},
			"note": "keep-me",
		},
	}

	newMeta, changed := removeSkillFromProjectMetadata(meta, "demo-skill")
	if !changed {
		t.Fatal("expected metadata change")
	}
	if len(newMeta.Tags) != 1 || newMeta.Tags[0] != "tag-a" {
		t.Fatalf("tags = %#v, want preserved", newMeta.Tags)
	}
	if newMeta.Type != "demo" {
		t.Fatalf("type = %q, want demo", newMeta.Type)
	}
	if newMeta.Extra["note"] != "keep-me" {
		t.Fatalf("extra.note = %#v, want keep-me", newMeta.Extra["note"])
	}

	skills, ok := newMeta.Extra["skills"].([]interface{})
	if !ok {
		t.Fatalf("skills type = %T, want []interface{}", newMeta.Extra["skills"])
	}
	if len(skills) != 1 {
		t.Fatalf("skills len = %d, want 1", len(skills))
	}
	entry, ok := skills[0].(map[string]interface{})
	if !ok || entry["code"] != "other-skill" {
		t.Fatalf("remaining skill = %#v, want other-skill", skills[0])
	}
}

func TestCreateProjectRecordsInitialActivitiesInOrder(t *testing.T) {
	database := setupTestDB(t)
	ctx := auth.WithContext(context.Background(), &types.Caller{
		Uin:   1,
		OrgID: 1,
		State: types.AuthStateSucc,
	}, &types.Trace{})

	seedReadyAssistant(t, database, "default", "默认队友", "默认队友")
	assistant := seedReadyAssistant(t, database, "analyst", "分析专家", "分析专家")
	user := &types.User{
		PublicID: "usr_member",
		Name:     "成员",
		Email:    "member@example.com",
		Phone:    "13800000002",
	}
	if err := database.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := database.Create(&types.UserOrg{
		Uin:    2,
		UserID: user.ID,
		OrgID:  1,
	}).Error; err != nil {
		t.Fatalf("create user org: %v", err)
	}

	service := NewProjectService(database, nil, nil, "test")
	project, err := service.CreateProject(ctx, &contract.CreateProjectRequest{
		Name: "手动创建项目",
		Members: []contract.MemberInput{
			{Type: "assistant", ID: assistant.PublicID},
			{Type: "user", ID: user.PublicID},
		},
		Metadata: map[string]interface{}{
			"extra": map[string]interface{}{
				"skills": []interface{}{
					map[string]interface{}{"code": "skill-alpha", "name": "Skill Alpha"},
					map[string]interface{}{"code": "skill-beta", "name": "Skill Beta"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	activities, err := infradb.ListProjectActivities(ctx, database, infradb.ProjectActivityListOptions{
		ProjectID: project.PublicID,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListProjectActivities failed: %v", err)
	}
	if len(activities) != 3 {
		t.Fatalf("activity count = %d, want 3", len(activities))
	}

	// 列表默认倒序；按时间正序验证创建项目 -> 队友 -> 技能。
	ordered := []*types.ProjectActivity{activities[2], activities[1], activities[0]}
	gotActions := []types.ProjectActivityAction{
		ordered[0].ActionType,
		ordered[1].ActionType,
		ordered[2].ActionType,
	}
	wantActions := []types.ProjectActivityAction{
		types.ProjectActivityActionProjectCreated,
		types.ProjectActivityActionParticipantsChanged,
		types.ProjectActivityActionSkillsChanged,
	}
	if !reflect.DeepEqual(gotActions, wantActions) {
		t.Fatalf("actions = %#v, want %#v", gotActions, wantActions)
	}
	if !ordered[0].CreatedAt.Before(ordered[1].CreatedAt) || !ordered[1].CreatedAt.Before(ordered[2].CreatedAt) {
		t.Fatalf("created_at order is not project -> participants -> skills: %v, %v, %v",
			ordered[0].CreatedAt, ordered[1].CreatedAt, ordered[2].CreatedAt)
	}
	if !reflect.DeepEqual(ordered[1].Payload.AddedAITeammateIDs, []string{assistant.PublicID}) {
		t.Fatalf("added ai teammate ids = %#v, want %s", ordered[1].Payload.AddedAITeammateIDs, assistant.PublicID)
	}
	if !reflect.DeepEqual(ordered[1].Payload.AddedMemberIDs, []string{user.PublicID}) {
		t.Fatalf("added member ids = %#v, want %s", ordered[1].Payload.AddedMemberIDs, user.PublicID)
	}
	if !reflect.DeepEqual(ordered[2].Payload.AddedSkillIDs, []string{"skill-alpha", "skill-beta"}) {
		t.Fatalf("added skill ids = %#v", ordered[2].Payload.AddedSkillIDs)
	}
}

func TestRemoveSkillFromProjectMetadata_MatchesNameCaseInsensitive(t *testing.T) {
	meta := types.ObjectMetadata{
		Extra: map[string]interface{}{
			"skills": []interface{}{
				map[string]interface{}{"code": "alpha", "name": "Demo Skill"},
			},
		},
	}

	_, changed := removeSkillFromProjectMetadata(meta, "demo skill")
	if !changed {
		t.Fatal("expected metadata change when matching display name")
	}
}

func TestRemoveSkillFromProjectMetadata_NoSkills(t *testing.T) {
	meta := types.ObjectMetadata{
		Extra: map[string]interface{}{"note": "only"},
	}

	_, changed := removeSkillFromProjectMetadata(meta, "demo-skill")
	if changed {
		t.Fatal("expected no change without skills array")
	}
}

func TestRemoveSkillFromProjectMetadata_NoMatch(t *testing.T) {
	meta := types.ObjectMetadata{
		Extra: map[string]interface{}{
			"skills": []interface{}{
				map[string]interface{}{"code": "other-skill", "name": "Other"},
			},
		},
	}

	_, changed := removeSkillFromProjectMetadata(meta, "demo-skill")
	if changed {
		t.Fatal("expected no change when skill not referenced")
	}
}

func setupProjectSkillReferenceDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	database, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	cleanup := func() {
		sqlDB.Close()
	}
	return database, mock, cleanup
}

func TestCleanupOrgProjectSkillReferences_UpdatesMatchingProjects(t *testing.T) {
	database, mock, cleanup := setupProjectSkillReferenceDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	columns := []string{
		"id", "created_at", "updated_at", "deleted_at", "public_id",
		"org_id", "owner_id", "name", "description", "objective", "status",
		"gitea_repo_full_name", "gitea_repo_id", "gitea_default_branch", "metadata",
	}
	metadata := []byte(`{"extra":{"skills":[{"code":"demo-skill","name":"Demo Skill"},{"code":"keep","name":"Keep"}]}}`)

	mock.ExpectQuery(`SELECT .* FROM "leros_project" WHERE \(org_id = \$1 AND deleted_at IS NULL\) AND \(EXISTS`).
		WithArgs(uint(100), "demo-skill", "demo-skill").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			1, now, now, nil, "prj_demo",
			100, 1, "Demo Project", "", "", "active",
			"", 0, "main", metadata,
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "leros_project" SET`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	updated, err := cleanupOrgProjectSkillReferences(ctx, database, 100, "demo-skill")
	if err != nil {
		t.Fatalf("cleanupOrgProjectSkillReferences failed: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
