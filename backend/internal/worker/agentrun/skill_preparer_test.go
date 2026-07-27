package agentrun

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	skillcatalog "github.com/insmtx/Leros/backend/internal/skill/catalog"
	agentrundomain "github.com/insmtx/Leros/backend/internal/worker/agentrun/domain"
	"github.com/insmtx/Leros/backend/pkg/leros"
)

func TestPluginSkillPreparerLinksSystemSkillsAndCleansRunView(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv(leros.EnvWorkspaceRoot, workspaceRoot)
	systemSkill := filepath.Join(workspaceRoot, ".leros", "skills", ".system", "review")
	if err := os.MkdirAll(systemSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(systemSkill, "SKILL.md"), []byte("---\nname: review\ndescription: review\n---\nReview.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	downloadURLRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		downloadURLRequests++
	}))
	defer server.Close()
	projectRoot := t.TempDir()
	prepared, cleanup, err := NewPluginSkillPreparer(server.URL, "").PrepareSkills(context.Background(), &agentrundomain.RunRequest{RunID: "run-1", Input: agentrundomain.InputContext{Messages: []agentrundomain.InputMessage{{Role: "user", Content: "/review the document"}}}}, WorkspacePreparation{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("PrepareSkills() error = %v", err)
	}
	link := filepath.Join(prepared, "review")
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected system Skill symlink, info=%v err=%v", info, err)
	}
	catalog, err := skillcatalog.NewCatalog(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Get("review"); err != nil {
		t.Fatalf("project catalog must resolve symlinked skill: %v", err)
	}
	if downloadURLRequests != 0 {
		t.Fatalf("built-in Skill must not request project download URLs, got %d", downloadURLRequests)
	}
	cleanup()
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup to remove run symlink, err=%v", err)
	}
}

func TestPluginSkillPreparerInstallsSkillAtWorkerRootAndRefreshesIt(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv(leros.EnvWorkspaceRoot, workspaceRoot)
	firstPackage := testSkillPackage(t, "xlsx", "first revision")
	secondPackage := testSkillPackage(t, "xlsx", "second revision")
	downloadURLRequests := 0
	packageDownloads := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/plugins/skills/download-urls":
			if got := request.Header.Get("Authorization"); got != "Bearer worker-token" {
				t.Fatalf("download URL authorization = %q", got)
			}
			downloadURLRequests++
			packages := [][]byte{firstPackage, secondPackage}
			if downloadURLRequests > len(packages) {
				t.Fatalf("unexpected extra download URL request")
			}
			index := downloadURLRequests - 1
			_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "data": map[string]any{"skills": []map[string]any{{"code": "xlsx", "revision": index + 1, "sha256": testPackageHash(packages[index]), "download_url": server.URL + "/package/" + strconv.Itoa(index+1)}}}})
		case "/package/1":
			packageDownloads++
			_, _ = writer.Write(firstPackage)
		case "/package/2":
			packageDownloads++
			_, _ = writer.Write(secondPackage)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	baseline := &skillBaselineCommitterStub{}
	preparer := NewPluginSkillPreparerWithBaseline(server.URL, "worker-token", baseline)
	firstRequest := &agentrundomain.RunRequest{RunID: "run-one", Plugins: []agentrundomain.PluginSnapshot{testPluginSkillSnapshot("xlsx", 1, firstPackage)}}
	firstView, firstCleanup, err := preparer.PrepareSkills(context.Background(), firstRequest, WorkspacePreparation{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("install first skill: %v", err)
	}
	defer firstCleanup()
	skillRoot := filepath.Join(workspaceRoot, ".leros", "skills", "xlsx")
	if got := readSkillBody(t, skillRoot); got != "first revision" {
		t.Fatalf("first installed skill body = %q", got)
	}
	firstHash := testPackageHash(firstPackage)
	manifestPath := filepath.Join(workspaceRoot, ".leros", "skills", ".seed-manifest")
	if got, err := os.ReadFile(manifestPath); err != nil || string(got) != "xlsx:"+firstHash+":1\n" {
		t.Fatalf("manifest = %q, err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".leros", "skills", ".plugins")); !os.IsNotExist(err) {
		t.Fatalf("legacy plugin cache must not be created, err=%v", err)
	}

	_, secondCleanup, err := preparer.PrepareSkills(context.Background(), &agentrundomain.RunRequest{RunID: "run-two", Plugins: firstRequest.Plugins, Input: agentrundomain.InputContext{Messages: []agentrundomain.InputMessage{{Role: "user", Content: "/xlsx reuse project Skill"}}}}, WorkspacePreparation{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("reuse installed skill: %v", err)
	}
	defer secondCleanup()
	if downloadURLRequests != 1 || packageDownloads != 1 {
		t.Fatalf("expected matching install to skip requests, urls=%d downloads=%d", downloadURLRequests, packageDownloads)
	}

	_, thirdCleanup, err := preparer.PrepareSkills(context.Background(), &agentrundomain.RunRequest{RunID: "run-three", Plugins: []agentrundomain.PluginSnapshot{testPluginSkillSnapshot("xlsx", 2, secondPackage)}}, WorkspacePreparation{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("update installed skill: %v", err)
	}
	defer thirdCleanup()
	if downloadURLRequests != 2 || packageDownloads != 2 {
		t.Fatalf("expected updated revision to download once, urls=%d downloads=%d", downloadURLRequests, packageDownloads)
	}
	if got := readSkillBody(t, skillRoot); got != "second revision" {
		t.Fatalf("updated installed skill body = %q", got)
	}
	if got := readSkillBody(t, filepath.Join(firstView, "xlsx")); got != "second revision" {
		t.Fatalf("existing run link must follow updated skill, got %q", got)
	}
	secondHash := testPackageHash(secondPackage)
	if got, err := os.ReadFile(manifestPath); err != nil || string(got) != "xlsx:"+secondHash+":2\n" {
		t.Fatalf("updated manifest = %q, err=%v", got, err)
	}
	if len(baseline.commits) != 2 ||
		len(baseline.commits[0]) != 1 || baseline.commits[0][0] != "xlsx" ||
		len(baseline.commits[1]) != 1 || baseline.commits[1][0] != "xlsx" {
		t.Fatalf("installed Skill baseline commits = %#v", baseline.commits)
	}
}

type skillBaselineCommitterStub struct {
	commits [][]string
}

func (s *skillBaselineCommitterStub) CommitInstalled(_ context.Context, codes []string) error {
	s.commits = append(s.commits, append([]string(nil), codes...))
	return nil
}

func TestPluginSkillPreparerSkipsSkillsWhenInstallManifestIsInvalid(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv(leros.EnvWorkspaceRoot, workspaceRoot)
	skillsRoot := filepath.Join(workspaceRoot, ".leros", "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsRoot, ".seed-manifest"), []byte("xlsx:not-a-hash:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packageBytes := testSkillPackage(t, "xlsx", "should not install")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	prepared, cleanup, err := NewPluginSkillPreparer(server.URL, "").PrepareSkills(context.Background(), &agentrundomain.RunRequest{RunID: "run-invalid", Plugins: []agentrundomain.PluginSnapshot{testPluginSkillSnapshot("xlsx", 1, packageBytes)}}, WorkspacePreparation{ProjectRoot: t.TempDir()})
	defer cleanup()
	if err != nil {
		t.Fatalf("invalid manifest must not fail run preparation: %v", err)
	}
	if requests != 0 {
		t.Fatalf("invalid manifest must fail before download, got %d requests", requests)
	}
	if _, err := os.Stat(filepath.Join(skillsRoot, "xlsx")); !os.IsNotExist(err) {
		t.Fatalf("invalid manifest must not install skill, err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(prepared, "xlsx")); !os.IsNotExist(err) {
		t.Fatalf("invalid manifest must not create run skill link, err=%v", err)
	}
}

func TestPluginSkillPreparerSkipsUnresolvedSkillWithoutFailingRun(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv(leros.EnvWorkspaceRoot, workspaceRoot)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/plugins/skills/download-urls" {
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "data": map[string]any{"skills": []any{}}})
	}))
	defer server.Close()
	prepared, cleanup, err := NewPluginSkillPreparer(server.URL, "worker-token").PrepareSkills(
		context.Background(),
		&agentrundomain.RunRequest{RunID: "run-unresolved", Plugins: []agentrundomain.PluginSnapshot{testPluginSkillSnapshot("xlsx", 1, testSkillPackage(t, "xlsx", "unused"))}},
		WorkspacePreparation{ProjectRoot: t.TempDir()},
	)
	defer cleanup()
	if err != nil {
		t.Fatalf("unresolved Skill must not fail run preparation: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(prepared, "xlsx")); !os.IsNotExist(err) {
		t.Fatalf("unresolved Skill must not create run link, err=%v", err)
	}
}

func TestPluginSkillPreparerFetchesMissingInvokedSkill(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv(leros.EnvWorkspaceRoot, workspaceRoot)
	packageBytes := testSkillPackage(t, "docx", "invoked skill")
	requests := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/plugins/skills/download-urls":
			requests++
			var body struct {
				SkillCodes []string `json:"skill_codes"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.SkillCodes) != 1 || body.SkillCodes[0] != "docx" {
				t.Fatalf("requested Skill codes = %#v", body.SkillCodes)
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "data": map[string]any{"skills": []map[string]any{{"code": "docx", "revision": 3, "sha256": testPackageHash(packageBytes), "download_url": server.URL + "/package"}}}})
		case "/package":
			_, _ = writer.Write(packageBytes)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	prepared, cleanup, err := NewPluginSkillPreparer(server.URL, "worker-token").PrepareSkills(
		context.Background(),
		&agentrundomain.RunRequest{RunID: "run-invoked", Input: agentrundomain.InputContext{Messages: []agentrundomain.InputMessage{{Role: "user", Content: "/docx create a report"}}}},
		WorkspacePreparation{ProjectRoot: t.TempDir()},
	)
	defer cleanup()
	if err != nil {
		t.Fatalf("prepare invoked Skill: %v", err)
	}
	if requests != 1 {
		t.Fatalf("download URL request count = %d, want 1", requests)
	}
	link := filepath.Join(prepared, "docx")
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("invoked Skill link = %v, %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".leros", "skills", "docx", "SKILL.md")); err != nil {
		t.Fatalf("installed invoked Skill missing: %v", err)
	}
}

func TestSkillInstallManifestIsStrictAndSorted(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), ".seed-manifest")
	hashA := testPackageHash([]byte("a"))
	hashB := testPackageHash([]byte("b"))
	entries := map[string]skillInstallRecord{
		"xlsx": {SHA256: hashB, Revision: 2},
		"docx": {SHA256: hashA, Revision: 1},
	}
	if err := writeSkillInstallManifest(manifestPath, entries); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	got, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "docx:" + hashA + ":1\nxlsx:" + hashB + ":2\n"
	if string(got) != want {
		t.Fatalf("manifest = %q, want %q", got, want)
	}
	if _, err := readSkillInstallManifest(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("missing manifest should be empty: %v", err)
	}
	for _, manifest := range []string{"xlsx:" + hashA + "\n", "xlsx:not-a-hash:1\n", "xlsx:" + hashA + ":0\n", "xlsx:" + hashA + ":1\nxlsx:" + hashB + ":2\n"} {
		path := filepath.Join(t.TempDir(), ".seed-manifest")
		if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readSkillInstallManifest(path); err == nil {
			t.Fatalf("expected invalid manifest %q to fail", manifest)
		}
	}
}

func testPluginSkillSnapshot(code string, revision int, packageBytes []byte) agentrundomain.PluginSnapshot {
	return agentrundomain.PluginSnapshot{
		PluginID:   "plugin_" + code,
		Code:       code,
		Kind:       "skill",
		Revision:   revision,
		Definition: []byte(fmt.Sprintf(`{"schema":"skill/v1","artifact":{"file_upload_id":"file_%s","sha256":"%s"}}`, code, testPackageHash(packageBytes))),
	}
}

func testSkillPackage(t *testing.T, name, body string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(file, "---\nname: %s\ndescription: test\n---\n%s\n", name, body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func testPackageHash(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func readSkillBody(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.Equal(line, []byte("first revision")) || bytes.Equal(line, []byte("second revision")) {
			return string(line)
		}
	}
	return ""
}
