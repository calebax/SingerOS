package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	eventbus "github.com/insmtx/Leros/backend/internal/infra/mq"
	"github.com/insmtx/Leros/backend/internal/skill/fetch"
	"github.com/insmtx/Leros/backend/internal/worker/protocol"
	"github.com/insmtx/Leros/backend/pkg/dm"
	"github.com/insmtx/Leros/backend/types"
)

type skillMarketplaceService struct {
	db        *gorm.DB
	publisher eventbus.Publisher
}

// NewSkillMarketplaceService 创建 Skill 市场服务。
func NewSkillMarketplaceService(db *gorm.DB, publisher eventbus.Publisher) contract.SkillMarketplaceService {
	return &skillMarketplaceService{db: db, publisher: publisher}
}

func (s *skillMarketplaceService) SearchSkillMarketplace(ctx context.Context, req *contract.SearchSkillMarketplaceRequest) (*contract.SearchSkillMarketplaceResponse, error) {
	if req.Limit <= 0 {
		req.Limit = 80
	}
	if req.Limit > 200 {
		req.Limit = 200
	}

	// 决定查询哪些源
	queryBuiltin, queryExternal := s.resolveSources(req.SourceTypes)

	keyword := strings.TrimSpace(req.Keyword)
	var externalQuery string

	if keyword == "" {
		if req.Category != "" {
			externalQuery = req.Category
		} else {
			externalQuery = "office"
		}
	} else {
		externalQuery = keyword
	}

	var (
		mu       sync.Mutex
		allItems []contract.SkillMarketplaceItemView
		warnings []contract.SkillSourceWarning
		wg       sync.WaitGroup
	)

	// 内置源：优先排在前面
	if queryBuiltin {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := s.searchBuiltin(ctx, req.Keyword, req.Category, req.Limit)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				warnings = append(warnings, contract.SkillSourceWarning{
					SourceType: "Leros",
					Message:    err.Error(),
				})
			} else {
				allItems = append(allItems, items...)
			}
		}()
	}

	// 外部源（skills.sh）
	if queryExternal {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metas, err := fetch.NewSkillsShSource().Search(ctx, externalQuery, req.Limit)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				warnings = append(warnings, contract.SkillSourceWarning{
					SourceType: "Skills.sh",
					Message:    err.Error(),
				})
			} else {
				for _, meta := range metas {
					allItems = append(allItems, metaToView(meta))
				}
			}
		}()
	}

	wg.Wait()

	// 首屏聚合：内置源优先，截断至 limit。
	if len(allItems) > req.Limit {
		allItems = allItems[:req.Limit]
	}

	return &contract.SearchSkillMarketplaceResponse{
		Items:    allItems,
		Warnings: warnings,
	}, nil
}

// resolveSources 根据 source_types 决定查询哪些源。
func (s *skillMarketplaceService) resolveSources(sourceTypes []string) (builtin, external bool) {
	if len(sourceTypes) == 0 {
		return true, true
	}
	for _, t := range sourceTypes {
		switch t {
		case "Leros":
			builtin = true
		case "Skills.sh":
			external = true
		}
	}
	return
}

// searchBuiltin 从数据库查询内置 Skill。
func (s *skillMarketplaceService) searchBuiltin(ctx context.Context, keyword, category string, limit int) ([]contract.SkillMarketplaceItemView, error) {
	items, err := infradb.SearchBuiltinSkills(ctx, s.db, keyword, category, limit)
	if err != nil {
		return nil, err
	}

	result := make([]contract.SkillMarketplaceItemView, 0, len(items))
	for _, item := range items {
		result = append(result, builtinItemToView(item))
	}
	return result, nil
}

func builtinItemToView(item types.BuiltinSkillMarketplaceItem) contract.SkillMarketplaceItemView {
	return contract.SkillMarketplaceItemView{
		SourceType:  "Leros",
		SkillID:     item.SkillID,
		Name:        item.Name,
		Description: item.Description,
		Version:     item.Version,
		Author:      item.Author,
		Category:    item.Category,
		Tags:        []string(item.Tags),
		Icon:        item.Icon,
		Installs:    item.Installs,
	}
}

func metaToView(meta fetch.SkillMeta) contract.SkillMarketplaceItemView {
	return contract.SkillMarketplaceItemView{
		SourceType:  meta.Source,
		SkillID:     meta.SkillID,
		Name:        meta.Name,
		Description: meta.Description,
		Version:     meta.Version,
		Author:      meta.Author,
		Category:    meta.Category,
		Tags:        meta.Tags,
		Icon:        meta.Icon,
		Installs:    meta.Installs,
	}
}

func (s *skillMarketplaceService) DownloadBuiltinSkill(ctx context.Context, skillID string) (*contract.SkillPackageDownload, error) {
	item, err := infradb.GetBuiltinSkillByID(ctx, s.db, skillID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("skill not found")
	}

	serverDir, err := infradb.ResolveSkillsServerDir()
	if err != nil {
		return nil, fmt.Errorf("resolve skills server dir: %w", err)
	}

	skillDir := filepath.Join(serverDir, skillID)
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); os.IsNotExist(err) {
		return nil, fmt.Errorf("skill %q found in DB but SKILL.md missing on disk", skillID)
	}

	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(zipSkillDir(ctx, pw, skillDir))
	}()

	return &contract.SkillPackageDownload{
		Reader:   pr,
		FileName: skillID + ".zip",
	}, nil
}

func zipSkillDir(ctx context.Context, w io.Writer, skillDir string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	return filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		relPath, err := filepath.Rel(skillDir, path)
		if err != nil {
			return err
		}

		zipPath := filepath.ToSlash(relPath)

		f, err := zw.Create(zipPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, file)
		file.Close()
		return err
	})
}

func (s *skillMarketplaceService) InstallSkill(ctx context.Context, req *contract.InstallSkillRequest) (*contract.InstallSkillResponse, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}

	workerID := uint(1)

	topic, err := dm.WorkerSkillSubject(caller.OrgID, workerID)
	if err != nil {
		return nil, fmt.Errorf("build skill topic: %w", err)
	}

	msg := protocol.SkillManagementMessage{
		ID:        fmt.Sprintf("%d-%d-%d", caller.OrgID, workerID, time.Now().UnixNano()),
		Type:      protocol.MessageTypeSkillManagement,
		CreatedAt: time.Now(),
		Route: protocol.RouteContext{
			OrgID:    caller.OrgID,
			WorkerID: workerID,
		},
		Body: protocol.SkillManagementBody{
			Action:  "install",
			Source:  strings.TrimSpace(req.Source),
			SkillID: strings.TrimSpace(req.SkillID),
		},
	}

	if err := s.publisher.Publish(ctx, topic, msg); err != nil {
		return nil, fmt.Errorf("publish skill install: %w", err)
	}

	return &contract.InstallSkillResponse{
		Status:  "accepted",
		Message: fmt.Sprintf("Skill install request queued for org %d, worker %d", caller.OrgID, workerID),
	}, nil
}

func (s *skillMarketplaceService) InstalledSkills(ctx context.Context, req *contract.InstalledSkillsRequest) (*contract.InstalledSkillsResponse, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}

	workerID := uint(1)

	topic, err := dm.WorkerSkillSubject(caller.OrgID, workerID)
	if err != nil {
		return nil, fmt.Errorf("build skill topic: %w", err)
	}

	msg := protocol.SkillManagementMessage{
		ID:        fmt.Sprintf("%d-%d-%d", caller.OrgID, workerID, time.Now().UnixNano()),
		Type:      protocol.MessageTypeSkillManagement,
		CreatedAt: time.Now(),
		Route: protocol.RouteContext{
			OrgID:    caller.OrgID,
			WorkerID: workerID,
		},
		Body: protocol.SkillManagementBody{
			Action: "list",
		},
	}

	reply, err := s.publisher.Request(ctx, topic, msg)
	if err != nil {
		return nil, fmt.Errorf("request skill list: %w", err)
	}

	var resp protocol.SkillManagementResponse
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal skill list response: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("skill list failed: %s", resp.Error)
	}

	// Convert response data to contract type
	var skills []contract.SkillInstalledItem
	if err := json.Unmarshal(resp.Data, &skills); err != nil {
		return nil, fmt.Errorf("unmarshal skill list items: %w", err)
	}

	return &contract.InstalledSkillsResponse{Skills: skills}, nil
}

func (s *skillMarketplaceService) UninstallSkill(ctx context.Context, req *contract.UninstallSkillRequest) (*contract.UninstallSkillResponse, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}

	workerID := uint(1)

	topic, err := dm.WorkerSkillSubject(caller.OrgID, workerID)
	if err != nil {
		return nil, fmt.Errorf("build skill topic: %w", err)
	}

	msg := protocol.SkillManagementMessage{
		ID:        fmt.Sprintf("%d-%d-%d", caller.OrgID, workerID, time.Now().UnixNano()),
		Type:      protocol.MessageTypeSkillManagement,
		CreatedAt: time.Now(),
		Route: protocol.RouteContext{
			OrgID:    caller.OrgID,
			WorkerID: workerID,
		},
		Body: protocol.SkillManagementBody{
			Action: "uninstall",
			Name:   strings.TrimSpace(req.Name),
		},
	}

	if err := s.publisher.Publish(ctx, topic, msg); err != nil {
		return nil, fmt.Errorf("publish skill uninstall: %w", err)
	}

	return &contract.UninstallSkillResponse{
		Status:  "accepted",
		Message: fmt.Sprintf("Skill uninstall request queued for org %d, worker %d", caller.OrgID, workerID),
	}, nil
}

func (s *skillMarketplaceService) GetSkillDetail(ctx context.Context, req *contract.SkillDetailRequest) (*contract.SkillDetailResponse, error) {
	source := strings.TrimSpace(req.Source)
	skillID := strings.TrimSpace(req.SkillID)

	switch source {
	case "Leros":
		return s.getLerosSkillDetail(ctx, skillID)
	case "installed":
		return s.getInstalledSkillDetail(ctx, skillID)
	default:
		return nil, fmt.Errorf("unsupported source: %s", source)
	}
}

// getLerosSkillDetail returns the full detail of a built-in marketplace skill.
func (s *skillMarketplaceService) getLerosSkillDetail(ctx context.Context, skillID string) (*contract.SkillDetailResponse, error) {
	item, err := infradb.GetBuiltinSkillByID(ctx, s.db, skillID)
	if err != nil {
		return nil, fmt.Errorf("query builtin skill: %w", err)
	}
	if item == nil {
		return nil, fmt.Errorf("skill %q not found", skillID)
	}

	serverDir, err := infradb.ResolveSkillsServerDir()
	if err != nil {
		return nil, fmt.Errorf("resolve skills server dir: %w", err)
	}

	skillMDPath := filepath.Join(serverDir, skillID, "SKILL.md")
	skillMD, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md for %q: %w", skillID, err)
	}

	// Collect files from the skill directory (include SKILL.md as the primary file).
	var files []string
	skillDir := filepath.Join(serverDir, skillID)
	files = append(files, "SKILL.md")
	if entries, readErr := os.ReadDir(skillDir); readErr == nil {
		for _, e := range entries {
			if e.IsDir() || e.Name() == "SKILL.md" {
				continue
			}
			files = append(files, e.Name())
		}
	}

	return &contract.SkillDetailResponse{
		SkillID:     item.SkillID,
		Source:      "Leros",
		Name:        item.Name,
		Description: item.Description,
		SkillMD:     stripFrontmatter(string(skillMD)),
		Version:     item.Version,
		Author:      item.Author,
		Category:    item.Category,
		Tags:        []string(item.Tags),
		Icon:        item.Icon,
		Installs:    item.Installs,
		Verified:    item.Verified,
		SourceType:  "Leros",
		Files:       files,
	}, nil
}

// getInstalledSkillDetail sends a NATS request to the worker for installed skill detail.
func (s *skillMarketplaceService) getInstalledSkillDetail(ctx context.Context, skillID string) (*contract.SkillDetailResponse, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}

	workerID := uint(1)

	topic, err := dm.WorkerSkillSubject(caller.OrgID, workerID)
	if err != nil {
		return nil, fmt.Errorf("build skill topic: %w", err)
	}

	msg := protocol.SkillManagementMessage{
		ID:        fmt.Sprintf("%d-%d-%d", caller.OrgID, workerID, time.Now().UnixNano()),
		Type:      protocol.MessageTypeSkillManagement,
		CreatedAt: time.Now(),
		Route: protocol.RouteContext{
			OrgID:    caller.OrgID,
			WorkerID: workerID,
		},
		Body: protocol.SkillManagementBody{
			Action: "detail",
			Name:   skillID,
		},
	}

	reply, err := s.publisher.Request(ctx, topic, msg)
	if err != nil {
		return nil, fmt.Errorf("request skill detail: %w", err)
	}

	var resp protocol.SkillManagementResponse
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal skill detail response: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("skill detail failed: %s", resp.Error)
	}

	var detail protocol.SkillDetailData
	if err := json.Unmarshal(resp.Data, &detail); err != nil {
		return nil, fmt.Errorf("unmarshal skill detail items: %w", err)
	}

	return &contract.SkillDetailResponse{
		SkillID:     detail.Name,
		Source:      "installed",
		Name:        detail.Name,
		Description: detail.Description,
		SkillMD:     stripFrontmatter(detail.SkillMD),
		Version:     detail.Version,
		Author:      detail.Source,
		Category:    detail.Category,
		Tags:        detail.Tags,
		Installs:    0,
		Verified:    detail.Trust == "trusted",
		SourceType:  detail.Source,
		Files:       detail.Files,
	}, nil
}

// stripFrontmatter removes YAML frontmatter (delimited by ---) from a SKILL.md file.
// Returns only the body content after the closing --- delimiter.
func stripFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			// Skip the closing --- line itself and any blank line immediately following it
			start := i + 1
			for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
				start++
			}
			return strings.Join(lines[start:], "\n")
		}
	}
	return content
}

var _ contract.SkillMarketplaceService = (*skillMarketplaceService)(nil)
