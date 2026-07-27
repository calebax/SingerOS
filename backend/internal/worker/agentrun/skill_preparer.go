package agentrun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	skilltoken "github.com/insmtx/Leros/backend/internal/skill"
	skillarchive "github.com/insmtx/Leros/backend/internal/skill/archive"
	agentrundomain "github.com/insmtx/Leros/backend/internal/worker/agentrun/domain"
	"github.com/insmtx/Leros/backend/pkg/leros"
	"github.com/ygpkg/yg-go/logs"
)

// SkillPreparer materializes the strictly scoped Skill view used by one run.
type SkillPreparer interface {
	PrepareSkills(context.Context, *agentrundomain.RunRequest, WorkspacePreparation) (string, func(), error)
}

// SkillBaselineCommitter records server-installed Skills as the local Git baseline.
type SkillBaselineCommitter interface {
	CommitInstalled(context.Context, []string) error
}

// PluginSkillPreparer installs project Skill bundles into the worker workspace
// and creates only symlinks in the project run view.
type PluginSkillPreparer struct {
	serverAddr        string
	authToken         string
	httpClient        *http.Client
	baselineCommitter SkillBaselineCommitter
}

type skillInstallRecord struct {
	SHA256   string
	Revision int
}

type skillDownloadURLResponse struct {
	Code        string `json:"code"`
	Revision    int    `json:"revision"`
	SHA256      string `json:"sha256"`
	DownloadURL string `json:"download_url"`
}

var skillInstallLocks sync.Map // map[string]*sync.Mutex, keyed by the worker skills root.

// NewPluginSkillPreparer creates the worker implementation. A zero server
// address is valid only when no project Skill artifact needs downloading.
func NewPluginSkillPreparer(serverAddr, authToken string) *PluginSkillPreparer {
	return NewPluginSkillPreparerWithBaseline(serverAddr, authToken, nil)
}

// NewPluginSkillPreparerWithBaseline creates a preparer that commits server
// installs so they cannot be mistaken for Run-created Skill changes.
func NewPluginSkillPreparerWithBaseline(
	serverAddr, authToken string,
	baselineCommitter SkillBaselineCommitter,
) *PluginSkillPreparer {
	return &PluginSkillPreparer{
		serverAddr: strings.TrimSpace(serverAddr), authToken: strings.TrimSpace(authToken),
		httpClient: &http.Client{}, baselineCommitter: baselineCommitter,
	}
}

func (p *PluginSkillPreparer) PrepareSkills(ctx context.Context, req *agentrundomain.RunRequest, workspace WorkspacePreparation) (string, func(), error) {
	if req == nil {
		return "", func() {}, fmt.Errorf("run request is required")
	}
	viewRoot, err := skillRunViewRoot(req, workspace)
	if err != nil {
		return "", func() {}, err
	}
	if err := os.MkdirAll(viewRoot, 0o755); err != nil {
		return "", func() {}, fmt.Errorf("create run skill directory: %w", err)
	}
	cleanup := func() { removeSkillLinks(viewRoot) }
	if err := linkSkillChildren(systemSkillsDir(), viewRoot); err != nil {
		cleanup()
		return "", cleanup, fmt.Errorf("link worker system skills: %w", err)
	}
	p.preparePluginSkills(ctx, req.Plugins, viewRoot)
	p.prepareInvokedSkills(ctx, req.Input.Messages, req.Plugins, viewRoot)
	return viewRoot, cleanup, nil
}

func (p *PluginSkillPreparer) preparePluginSkills(ctx context.Context, snapshots []agentrundomain.PluginSnapshot, viewRoot string) {
	skillsRoot, err := leros.JoinWorkspace(".leros", "skills")
	if err != nil {
		logs.WarnContextf(ctx, "resolve worker Skill directory failed: %v", err)
		return
	}
	lockValue, _ := skillInstallLocks.LoadOrStore(skillsRoot, &sync.Mutex{})
	installLock := lockValue.(*sync.Mutex)
	installLock.Lock()
	defer installLock.Unlock()
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		logs.WarnContextf(ctx, "create worker Skill directory failed: %v", err)
		return
	}
	manifestPath := filepath.Join(skillsRoot, ".seed-manifest")
	installed, err := readSkillInstallManifest(manifestPath)
	if err != nil {
		logs.WarnContextf(ctx, "read worker Skill install manifest failed: %v", err)
		return
	}
	pending := make(map[string]struct{})
	available := make(map[string]string)
	for _, snapshot := range sortedPluginSnapshots(snapshots) {
		if !strings.EqualFold(snapshot.Kind, "skill") {
			continue
		}
		code, err := organizationSkillName(snapshot.Code)
		if err != nil || snapshot.Revision <= 0 {
			logs.WarnContextf(ctx, "skip invalid project Skill %q: code=%v revision=%d", snapshot.Code, err, snapshot.Revision)
			continue
		}
		content := filepath.Join(skillsRoot, code)
		if record, ok := installed[code]; ok && record.Revision == snapshot.Revision && hasSkillDocument(content) {
			available[code] = content
			continue
		}
		pending[code] = struct{}{}
	}
	if len(pending) > 0 {
		codes := make([]string, 0, len(pending))
		for code := range pending {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		downloads, err := p.resolveDownloadURLs(ctx, codes)
		if err != nil {
			logs.WarnContextf(ctx, "resolve project Skill download URLs failed: %v", err)
		} else {
			installedCodes := make([]string, 0, len(codes))
			for _, code := range codes {
				download, ok := downloads[code]
				if !ok {
					logs.WarnContextf(ctx, "skip project Skill %q: server returned no download URL", code)
					continue
				}
				content, err := installSkillFromURL(ctx, skillsRoot, code, download)
				if err != nil {
					logs.WarnContextf(ctx, "skip project Skill %q: %v", code, err)
					continue
				}
				installed[code] = skillInstallRecord{SHA256: download.SHA256, Revision: download.Revision}
				available[code] = content
				installedCodes = append(installedCodes, code)
			}
			if err := writeSkillInstallManifest(manifestPath, installed); err != nil {
				logs.WarnContextf(ctx, "write worker Skill install manifest failed: %v", err)
			} else {
				p.commitInstalledBaseline(ctx, installedCodes)
			}
		}
	}
	for code, content := range available {
		if err := replaceRunSkillLink(content, filepath.Join(viewRoot, code)); err != nil {
			logs.WarnContextf(ctx, "skip project Skill %q link: %v", code, err)
		}
	}
}

// prepareInvokedSkills installs the latest organization Skill only when a user
// explicitly invokes one that is neither built in nor part of the project.
func (p *PluginSkillPreparer) prepareInvokedSkills(ctx context.Context, messages []agentrundomain.InputMessage, snapshots []agentrundomain.PluginSnapshot, viewRoot string) {
	projectSkills := projectSkillCodes(snapshots)
	missing := make(map[string]struct{})
	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		for _, token := range skilltoken.ParseTokensOnly(message.Content) {
			code, err := organizationSkillName(token)
			if err != nil {
				logs.WarnContextf(ctx, "skip invalid invoked Skill %q: %v", token, err)
				continue
			}
			if projectSkills[code] || isSystemSkill(code) {
				continue
			}
			if _, err := os.Lstat(filepath.Join(viewRoot, code)); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				logs.WarnContextf(ctx, "inspect invoked Skill %q in run view: %v", code, err)
				continue
			}
			missing[code] = struct{}{}
		}
	}
	if len(missing) == 0 {
		return
	}
	codes := make([]string, 0, len(missing))
	for code := range missing {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	p.installLatestSkills(ctx, codes, viewRoot)
}

func projectSkillCodes(snapshots []agentrundomain.PluginSnapshot) map[string]bool {
	codes := make(map[string]bool)
	for _, snapshot := range snapshots {
		if !strings.EqualFold(snapshot.Kind, "skill") {
			continue
		}
		code, err := organizationSkillName(snapshot.Code)
		if err == nil {
			codes[code] = true
		}
	}
	return codes
}

func isSystemSkill(code string) bool {
	return hasSkillDocument(filepath.Join(systemSkillsDir(), code))
}

func (p *PluginSkillPreparer) installLatestSkills(ctx context.Context, codes []string, viewRoot string) {
	skillsRoot, err := leros.JoinWorkspace(".leros", "skills")
	if err != nil {
		logs.WarnContextf(ctx, "resolve worker Skill directory for invoked Skills failed: %v", err)
		return
	}
	lockValue, _ := skillInstallLocks.LoadOrStore(skillsRoot, &sync.Mutex{})
	installLock := lockValue.(*sync.Mutex)
	installLock.Lock()
	defer installLock.Unlock()
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		logs.WarnContextf(ctx, "create worker Skill directory for invoked Skills failed: %v", err)
		return
	}
	manifestPath := filepath.Join(skillsRoot, ".seed-manifest")
	installed, err := readSkillInstallManifest(manifestPath)
	if err != nil {
		logs.WarnContextf(ctx, "read worker Skill install manifest for invoked Skills failed: %v", err)
		return
	}
	downloads, err := p.resolveDownloadURLs(ctx, codes)
	if err != nil {
		logs.WarnContextf(ctx, "resolve invoked Skill download URLs failed: %v", err)
		return
	}
	changed := false
	installedCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		download, ok := downloads[code]
		if !ok {
			logs.WarnContextf(ctx, "skip invoked Skill %q: server returned no download URL", code)
			continue
		}
		content, err := installSkillFromURL(ctx, skillsRoot, code, download)
		if err != nil {
			logs.WarnContextf(ctx, "skip invoked Skill %q: %v", code, err)
			continue
		}
		installed[code] = skillInstallRecord{SHA256: download.SHA256, Revision: download.Revision}
		changed = true
		installedCodes = append(installedCodes, code)
		if err := replaceRunSkillLink(content, filepath.Join(viewRoot, code)); err != nil {
			logs.WarnContextf(ctx, "skip invoked Skill %q link: %v", code, err)
		}
	}
	if changed {
		if err := writeSkillInstallManifest(manifestPath, installed); err != nil {
			logs.WarnContextf(ctx, "write worker Skill install manifest for invoked Skills failed: %v", err)
		} else {
			p.commitInstalledBaseline(ctx, installedCodes)
		}
	}
}

func (p *PluginSkillPreparer) commitInstalledBaseline(ctx context.Context, codes []string) {
	if p == nil || p.baselineCommitter == nil || len(codes) == 0 {
		return
	}
	if err := p.baselineCommitter.CommitInstalled(ctx, codes); err != nil {
		logs.WarnContextf(ctx, "commit installed Server Skills as baseline failed: %v", err)
	}
}

func (p *PluginSkillPreparer) resolveDownloadURLs(ctx context.Context, codes []string) (map[string]skillDownloadURLResponse, error) {
	if p == nil || strings.TrimSpace(p.serverAddr) == "" {
		return nil, fmt.Errorf("server address is required")
	}
	body, err := json.Marshal(struct {
		SkillCodes []string `json:"skill_codes"`
	}{SkillCodes: codes})
	if err != nil {
		return nil, fmt.Errorf("encode skill download URL request: %w", err)
	}
	base := strings.TrimRight(p.serverAddr, "/")
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/plugins/skills/download-urls", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create skill download URL request: %w", err)
	}
	if p.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.authToken)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request skill download URLs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("request skill download URLs: unexpected status %s", resp.Status)
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Skills []skillDownloadURLResponse `json:"skills"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2_000_000)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode skill download URL response: %w", err)
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("skill download URL request failed with code %d", payload.Code)
	}
	result := make(map[string]skillDownloadURLResponse, len(payload.Data.Skills))
	for _, item := range payload.Data.Skills {
		code, err := organizationSkillName(item.Code)
		sha, shaErr := normalizedSHA256(item.SHA256)
		if err != nil || shaErr != nil || item.Revision <= 0 || strings.TrimSpace(item.DownloadURL) == "" {
			continue
		}
		item.Code, item.SHA256 = code, sha
		result[code] = item
	}
	return result, nil
}

func installSkillFromURL(ctx context.Context, skillsRoot, code string, download skillDownloadURLResponse) (string, error) {
	artifactSHA, err := normalizedSHA256(download.SHA256)
	if err != nil {
		return "", fmt.Errorf("invalid Skill artifact sha256: %w", err)
	}
	temp, err := os.MkdirTemp(skillsRoot, ".skill-install-*")
	if err != nil {
		return "", fmt.Errorf("create Skill install temp: %w", err)
	}
	defer os.RemoveAll(temp)
	packagePath := filepath.Join(temp, "package.zip")
	if err := downloadArtifact(ctx, download.DownloadURL, packagePath); err != nil {
		return "", err
	}
	bytes, err := os.ReadFile(packagePath)
	if err != nil {
		return "", fmt.Errorf("read downloaded skill package: %w", err)
	}
	if err := os.Remove(packagePath); err != nil {
		return "", fmt.Errorf("remove downloaded skill package: %w", err)
	}
	hash := sha256.Sum256(bytes)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), artifactSHA) {
		return "", fmt.Errorf("downloaded skill package sha256 mismatch")
	}
	if err := skillarchive.Extract(bytes, temp); err != nil {
		return "", fmt.Errorf("validate and extract skill package: %w", err)
	}
	if !hasSkillDocument(temp) {
		return "", fmt.Errorf("downloaded skill package does not contain SKILL.md")
	}
	content := filepath.Join(skillsRoot, code)
	if err := replaceInstalledSkill(temp, content); err != nil {
		return "", err
	}
	return content, nil
}

func downloadArtifact(ctx context.Context, downloadURL, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create skill download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download skill package: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download skill package: unexpected status %s", resp.Status)
	}
	if resp.ContentLength > skillarchive.MaxPackageBytes {
		return fmt.Errorf("skill package exceeds %d byte limit", skillarchive.MaxPackageBytes)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create skill download temp: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(resp.Body, skillarchive.MaxPackageBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write skill package: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close skill package: %w", closeErr)
	}
	if written > skillarchive.MaxPackageBytes {
		return fmt.Errorf("skill package exceeds %d byte limit", skillarchive.MaxPackageBytes)
	}
	return nil
}

func skillRunViewRoot(req *agentrundomain.RunRequest, workspace WorkspacePreparation) (string, error) {
	runID, err := safePathID(req.RunID)
	if err != nil {
		return "", fmt.Errorf("invalid run id: %w", err)
	}
	if workspace.ProjectRoot != "" {
		return filepath.Join(workspace.ProjectRoot, ".leros", "skills", "runs", runID), nil
	}
	return leros.JoinWorkspace(".leros", "skills", "runs", runID)
}

func systemSkillsDir() string {
	dir, _ := leros.JoinWorkspace(".leros", "skills", ".system")
	return dir
}

func normalizedSHA256(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != sha256.Size*2 || !isHex(value) {
		return "", fmt.Errorf("must be a %d-character hexadecimal value", sha256.Size*2)
	}
	return value, nil
}

func organizationSkillName(value string) (string, error) {
	name, err := safeSkillName(value)
	if err != nil {
		return "", err
	}
	if name == "runs" || name == ".system" || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("reserved skill name %q", name)
	}
	return name, nil
}

func hasSkillDocument(root string) bool {
	info, err := os.Stat(filepath.Join(root, "SKILL.md"))
	return err == nil && !info.IsDir()
}

func readSkillInstallManifest(path string) (map[string]skillInstallRecord, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]skillInstallRecord), nil
	}
	if err != nil {
		return nil, err
	}
	entries := make(map[string]skillInstallRecord)
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("line %d must be skill_name:sha256:revision", lineNumber+1)
		}
		name, err := organizationSkillName(parts[0])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber+1, err)
		}
		hash, err := normalizedSHA256(parts[1])
		if err != nil {
			return nil, fmt.Errorf("line %d has invalid sha256", lineNumber+1)
		}
		revision, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil || revision <= 0 {
			return nil, fmt.Errorf("line %d has invalid revision", lineNumber+1)
		}
		if _, exists := entries[name]; exists {
			return nil, fmt.Errorf("line %d duplicates skill %q", lineNumber+1, name)
		}
		entries[name] = skillInstallRecord{SHA256: hash, Revision: revision}
	}
	return entries, nil
}

func writeSkillInstallManifest(path string, entries map[string]skillInstallRecord) error {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var builder strings.Builder
	for _, name := range names {
		entry := entries[name]
		fmt.Fprintf(&builder, "%s:%s:%d\n", name, strings.ToLower(entry.SHA256), entry.Revision)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(builder.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func replaceInstalledSkill(temp, destination string) error {
	backup := destination + ".backup"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove stale skill install backup: %w", err)
	}
	if err := os.Rename(destination, backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("back up installed skill: %w", err)
	}
	if err := os.Rename(temp, destination); err != nil {
		if restoreErr := os.Rename(backup, destination); restoreErr != nil && !os.IsNotExist(restoreErr) {
			return fmt.Errorf("promote installed skill: %w; restore previous skill: %v", err, restoreErr)
		}
		return fmt.Errorf("promote installed skill: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove replaced skill: %w", err)
	}
	return nil
}

func isHex(value string) bool {
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func linkSkillChildren(source, destination string) error {
	entries, err := os.ReadDir(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, err := safeSkillName(entry.Name())
		if err != nil {
			continue
		}
		if err := replaceRunSkillLink(filepath.Join(source, name), filepath.Join(destination, name)); err != nil {
			return err
		}
	}
	return nil
}

func replaceRunSkillLink(source, target string) error {
	info, err := os.Lstat(target)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refuse to replace non-symlink %s", target)
		}
		if err := os.Remove(target); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(source, target)
}

func removeSkillLinks(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(path)
		}
	}
	_ = os.Remove(root)
}

func safeSkillName(value string) (string, error) { return safePathID(value) }

func safePathID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || filepath.IsAbs(value) || strings.ContainsAny(value, "/\\") {
		return "", fmt.Errorf("invalid path identifier %q", value)
	}
	return value, nil
}

func sortedPluginSnapshots(values []agentrundomain.PluginSnapshot) []agentrundomain.PluginSnapshot {
	result := append([]agentrundomain.PluginSnapshot(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result
}
