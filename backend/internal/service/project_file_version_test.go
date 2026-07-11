package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/config"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/infra/filestore"
	"github.com/insmtx/Leros/backend/pkg/leros"
	"github.com/insmtx/Leros/backend/types"
)

func TestProjectFileVersionQueriesAndDownloads(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(
		&types.Project{},
		&types.FileUpload{},
		&types.ProjectFile{},
		&types.Session{},
		&types.WorkerDeployment{},
		&types.Resource{},
		&types.ResourceBinding{},
	); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := filestore.Init(&config.StorageConfig{
		Driver:   "local",
		LocalDir: t.TempDir(),
		Bucket:   "dev-bucket",
	}); err != nil {
		t.Fatalf("init filestore: %v", err)
	}

	project := &types.Project{PublicID: "prj_versions", OrgID: 1, OwnerID: 1, Name: "versions", Status: "active"}
	if err := database.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectResource := &types.Resource{OrgID: 1, Uin: 1, Type: types.ResourceTypeProject, BizID: project.ID}
	if err := infradb.CreateResource(context.Background(), database, projectResource); err != nil {
		t.Fatalf("create project resource: %v", err)
	}
	ownerUin := uint(1)
	if err := infradb.CreateResourceBinding(context.Background(), database, &types.ResourceBinding{
		OrgID: 1, Uin: &ownerUin, ResourceID: projectResource.ID, Role: types.ResourceRoleOwner,
	}); err != nil {
		t.Fatalf("create project resource binding: %v", err)
	}

	contents := []string{"version one", "version two"}
	created := make([]*types.ProjectFile, 0, len(contents))
	for i, content := range contents {
		var projectFile *types.ProjectFile
		err := database.Transaction(func(tx *gorm.DB) error {
			fileUpload, err := filestore.Upload(setupTestContextWithCaller(t), tx, filestore.UploadParams{
				Data:         []byte(content),
				Filename:     "report.md",
				OriginalName: "report.md",
				MimeType:     "text/markdown",
				OrgID:        1,
				OwnerID:      1,
				ObjectKey:    fmt.Sprintf("projects/1/prj_versions/artifacts/report-v%d.md", i+1),
				Purpose:      filestore.PurposeArtifact,
			})
			if err != nil {
				return err
			}
			projectFile = &types.ProjectFile{
				FilePublicID: fileUpload.PublicID,
				OrgID:        1,
				ProjectID:    project.ID,
				TaskID:       10,
				ResourceID:   fileUpload.ID,
				ResourceType: types.ProjectFileResourceTypeArtifact,
				Uin:          1,
				RelativePath: "artifacts/report.md",
			}
			return infradb.CreateProjectFileVersion(setupTestContextWithCaller(t), tx, projectFile)
		})
		if err != nil {
			t.Fatalf("create version %d: %v", i+1, err)
		}
		artifactResource := &types.Resource{
			OrgID: 1,
			Uin:   1,
			Type:  types.ResourceTypeArtifact,
			BizID: projectFile.ID,
		}
		if err := infradb.CreateResource(context.Background(), database, artifactResource); err != nil {
			t.Fatalf("create artifact resource %d: %v", i+1, err)
		}
		ownerUin := uint(1)
		if err := infradb.CreateResourceBinding(context.Background(), database, &types.ResourceBinding{
			OrgID: 1, Uin: &ownerUin, ResourceID: artifactResource.ID, Role: types.ResourceRoleOwner,
		}); err != nil {
			t.Fatalf("create artifact resource binding %d: %v", i+1, err)
		}
		created = append(created, projectFile)
	}

	service := &projectService{db: database, perm: NewPermissionService(database), inferrer: NewDefaultAssistantInferrer(1)}
	ctx := setupTestContextWithCaller(t)
	tree, err := service.GetProjectFileTree(ctx, project.PublicID, string(types.ProjectFileResourceTypeArtifact), "")
	if err != nil {
		t.Fatalf("get project file tree: %v", err)
	}
	if len(tree) != 1 || tree[0].PublicID != created[1].FilePublicID || tree[0].VersionNo != 2 {
		t.Fatalf("project file tree = %#v", tree)
	}

	versions, err := service.GetProjectFileVersions(ctx, project.PublicID, created[0].FilePublicID)
	if err != nil {
		t.Fatalf("get project file versions: %v", err)
	}
	if versions.CurrentFilePublicID != created[1].FilePublicID || len(versions.Items) != 2 {
		t.Fatalf("versions = %#v", versions)
	}
	if versions.Items[0].VersionNo != 2 || versions.Items[1].VersionNo != 1 {
		t.Fatalf("version order = %#v", versions.Items)
	}

	reader, contentType, size, err := service.DownloadProjectFileByPublicID(ctx, project.PublicID, created[0].FilePublicID)
	if err != nil {
		t.Fatalf("download first version: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read first version: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close first version: %v", err)
	}
	if string(data) != contents[0] || contentType != "text/markdown" || size != int64(len(contents[0])) {
		t.Fatalf("download = %q, %q, %d", data, contentType, size)
	}

	reader, _, _, err = service.DownloadProjectFile(ctx, project.PublicID, "artifacts/report.md")
	if err != nil {
		t.Fatalf("download latest by path: %v", err)
	}
	latestData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read latest version: %v", err)
	}
	_ = reader.Close()
	if strings.TrimSpace(string(latestData)) != contents[1] {
		t.Fatalf("latest content = %q", latestData)
	}

	workspaceRoot := t.TempDir()
	t.Setenv(leros.EnvWorkspaceRoot, workspaceRoot)
	repoDir := filepath.Join(workspaceRoot, "projects", "1", project.PublicID, "repo")
	artifactPath := filepath.Join(repoDir, "artifacts", "report.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("create artifact directory: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte(contents[1]), 0o644); err != nil {
		t.Fatalf("write current artifact: %v", err)
	}
	runProjectFileGitCommand(t, repoDir, "init")
	runProjectFileGitCommand(t, repoDir, "add", "--all")
	runProjectFileGitCommand(t, repoDir, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "initial")

	restored, err := service.RestoreProjectFileVersion(ctx, project.PublicID, created[0].FilePublicID)
	if err != nil {
		t.Fatalf("restore first version: %v", err)
	}
	if restored.VersionNo != 3 || restored.InitialFilePublicID != created[0].FilePublicID {
		t.Fatalf("restored node = %#v", restored)
	}
	restoredData, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read restored workspace file: %v", err)
	}
	if string(restoredData) != contents[0] {
		t.Fatalf("restored workspace content = %q", restoredData)
	}

	// Latest download should return restored content
	reader, _, _, err = service.DownloadProjectFile(ctx, project.PublicID, "artifacts/report.md")
	if err != nil {
		t.Fatalf("download latest after restore: %v", err)
	}
	latestRestored, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read latest after restore: %v", err)
	}
	_ = reader.Close()
	if string(latestRestored) != contents[0] {
		t.Fatalf("latest content after restore = %q", latestRestored)
	}

	// Old version v1 should still be downloadable with its original public_id
	reader, _, _, err = service.DownloadProjectFileByPublicID(ctx, project.PublicID, created[0].FilePublicID)
	if err != nil {
		t.Fatalf("download old v1 after restore: %v", err)
	}
	oldV1Data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read old v1 after restore: %v", err)
	}
	_ = reader.Close()
	if string(oldV1Data) != contents[0] {
		t.Fatalf("old v1 content after restore = %q", oldV1Data)
	}
	reader, _, _, err = service.DownloadProjectFileByPublicID(ctx, project.PublicID, created[1].FilePublicID)
	if err != nil {
		t.Fatalf("download old v2 after restore: %v", err)
	}
	oldV2Data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read old v2 after restore: %v", err)
	}
	_ = reader.Close()
	if string(oldV2Data) != contents[1] {
		t.Fatalf("old v2 content after restore = %q", oldV2Data)
	}

	versions, err = service.GetProjectFileVersions(ctx, project.PublicID, restored.PublicID)
	if err != nil {
		t.Fatalf("get restored versions: %v", err)
	}
	if versions.CurrentFilePublicID != restored.PublicID || len(versions.Items) != 3 {
		t.Fatalf("restored versions = %#v", versions)
	}
	versions, err = service.GetProjectFileVersions(ctx, project.PublicID, created[1].FilePublicID)
	if err != nil {
		t.Fatalf("get old v2 versions after restore: %v", err)
	}
	if versions.CurrentFilePublicID != restored.PublicID || len(versions.Items) != 3 {
		t.Fatalf("old v2 versions after restore = %#v", versions)
	}
}

func runProjectFileGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
