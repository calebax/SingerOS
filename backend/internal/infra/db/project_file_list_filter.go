package db

import (
	"context"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/workspace"
	"github.com/insmtx/Leros/backend/types"
)

// ProjectFileListFilter scopes project file list queries.
type ProjectFileListFilter struct {
	ResourceType string
	TaskID       uint
	NodeType     string
	FileExt      string
}

// ValidProjectFileExtFilter reports whether file_ext is a supported filter key.
func ValidProjectFileExtFilter(fileExt string) bool {
	switch strings.TrimSpace(fileExt) {
	case "pdf", "docx", "xlsx", "pptx", "md", "image", "text":
		return true
	default:
		return false
	}
}

// ListProjectFilesFiltered returns project files and folders for one filter scope.
func ListProjectFilesFiltered(
	ctx context.Context,
	db *gorm.DB,
	orgID uint,
	projectID uint,
	filter ProjectFileListFilter,
) ([]types.ProjectFile, error) {
	nodeType := strings.TrimSpace(filter.NodeType)
	fileExt := strings.TrimSpace(filter.FileExt)

	if nodeType == string(types.ProjectFileNodeTypeFolder) {
		folders, err := ListProjectFolderNodes(ctx, db, orgID, projectID, filter.TaskID, filter.ResourceType)
		if err != nil {
			return nil, err
		}
		files, err := listLatestProjectFileRecords(ctx, db, orgID, projectID, filter.TaskID, filter.ResourceType)
		if err != nil {
			return nil, err
		}
		files = filterFilesUnderFolderPaths(files, folders)
		return append(folders, files...), nil
	}

	files, err := listLatestProjectFileRecords(ctx, db, orgID, projectID, filter.TaskID, filter.ResourceType)
	if err != nil {
		return nil, err
	}
	if fileExt != "" {
		files = filterProjectFilesByExtGroup(files, fileExt)
	}
	if nodeType == string(types.ProjectFileNodeTypeFile) {
		return files, nil
	}

	folders, err := ListProjectFolderNodes(ctx, db, orgID, projectID, filter.TaskID, filter.ResourceType)
	if err != nil {
		return nil, err
	}
	if fileExt != "" {
		folders = filterFoldersForFiles(folders, files)
	}
	return append(folders, files...), nil
}

func listLatestProjectFileRecords(
	ctx context.Context,
	db *gorm.DB,
	orgID uint,
	projectID uint,
	taskID uint,
	resourceType string,
) ([]types.ProjectFile, error) {
	var files []types.ProjectFile
	base := db.WithContext(ctx).Model(&types.ProjectFile{}).
		Where("org_id = ? AND project_id = ?", orgID, projectID).
		Where("node_type = ? OR node_type = ''", types.ProjectFileNodeTypeFile)
	if taskID != 0 {
		base = base.Where("task_id = ?", taskID)
	}
	if resourceType != "" {
		base = base.Where("resource_type = ?", resourceType)
	} else {
		base = base.Where("resource_type != ?", types.ProjectFileResourceTypePlan)
	}

	latest := base.Session(&gorm.Session{}).
		Select("initial_file_public_id, MAX(version_no) AS version_no").
		Group("initial_file_public_id")
	table := types.TableNameProjectFile
	query := base.
		Joins(
			"JOIN (?) AS latest ON latest.initial_file_public_id = "+table+".initial_file_public_id AND latest.version_no = "+table+".version_no",
			latest,
		).
		Order(table + ".created_at DESC, " + table + ".id DESC")
	if err := query.Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func filterProjectFilesByExtGroup(files []types.ProjectFile, fileExt string) []types.ProjectFile {
	filtered := make([]types.ProjectFile, 0, len(files))
	for _, file := range files {
		if matchesProjectFileExtGroup(file.RelativePath, fileExt) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func matchesProjectFileExtGroup(relativePath, fileExt string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(relativePath)))
	switch strings.TrimSpace(fileExt) {
	case "pdf":
		return ext == ".pdf"
	case "docx":
		return ext == ".docx" || ext == ".doc"
	case "xlsx":
		return ext == ".xlsx" || ext == ".xls" || ext == ".csv"
	case "pptx":
		return ext == ".pptx" || ext == ".ppt"
	case "md":
		return ext == ".md" || ext == ".markdown"
	case "image":
		return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
	case "text":
		return ext == ".txt" || ext == ".json" || ext == ".yaml" || ext == ".yml" ||
			ext == ".log" || ext == ".html" || ext == ".htm"
	default:
		return true
	}
}

func folderPathsForFile(relativePath string) []string {
	normalized, err := workspace.NormalizeRelativePath(relativePath)
	if err != nil || normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, "/")
	if len(parts) <= 1 {
		return nil
	}
	paths := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		paths = append(paths, strings.Join(parts[:i], "/")+"/")
	}
	return paths
}

func filterFoldersForFiles(folders []types.ProjectFile, files []types.ProjectFile) []types.ProjectFile {
	if len(files) == 0 {
		return nil
	}

	needed := make(map[string]struct{})
	for _, file := range files {
		for _, folderPath := range folderPathsForFile(file.RelativePath) {
			needed[folderPath] = struct{}{}
		}
	}

	filtered := make([]types.ProjectFile, 0, len(folders))
	for _, folder := range folders {
		normalized, err := normalizeFolderRelativePath(folder.RelativePath)
		if err != nil {
			continue
		}
		if _, ok := needed[normalized]; ok {
			filtered = append(filtered, folder)
		}
	}
	return filtered
}

func filterFilesUnderFolderPaths(files []types.ProjectFile, folders []types.ProjectFile) []types.ProjectFile {
	if len(folders) == 0 {
		return nil
	}

	prefixes := make([]string, 0, len(folders))
	for _, folder := range folders {
		normalized, err := normalizeFolderRelativePath(folder.RelativePath)
		if err != nil || normalized == "" {
			continue
		}
		prefixes = append(prefixes, strings.TrimSuffix(normalized, "/"))
	}
	if len(prefixes) == 0 {
		return nil
	}

	filtered := make([]types.ProjectFile, 0, len(files))
	for _, file := range files {
		normalized, err := workspace.NormalizeRelativePath(file.RelativePath)
		if err != nil || normalized == "" {
			continue
		}
		for _, prefix := range prefixes {
			if filePathUnderFolderPrefix(normalized, prefix) {
				filtered = append(filtered, file)
				break
			}
		}
	}
	return filtered
}

func filePathUnderFolderPrefix(filePath, folderPrefix string) bool {
	folderPrefix = strings.TrimSuffix(strings.TrimSpace(folderPrefix), "/")
	if folderPrefix == "" {
		return false
	}
	return filePath == folderPrefix || strings.HasPrefix(filePath, folderPrefix+"/")
}

func normalizeFolderRelativePath(relativePath string) (string, error) {
	normalized, err := workspace.NormalizeRelativePath(relativePath)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		return "", nil
	}
	if !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}
	return normalized, nil
}
