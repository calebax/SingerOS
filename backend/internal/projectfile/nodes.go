package projectfile

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ygpkg/yg-go/encryptor/snowflake"
	"gorm.io/gorm"

	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/workspace"
	"github.com/insmtx/Leros/backend/types"
)

const taskUploadScopeSegment = "_task"

// BindUserUploadParams links one uploaded file into a project tree.
type BindUserUploadParams struct {
	OrgID         uint
	ProjectID     uint
	TaskID        *uint
	TaskPublicID  string
	Uin           uint
	FileUpload    *types.FileUpload
	DisplayName   string
	RelativePath  string
}

// BindUserUploadToProject creates folder ancestors and a file node for one upload.
func BindUserUploadToProject(ctx context.Context, db *gorm.DB, params BindUserUploadParams) (*types.ProjectFile, error) {
	if params.FileUpload == nil {
		return nil, fmt.Errorf("file upload is required")
	}
	relativePath, err := ResolveUserUploadRelativePath(
		params.DisplayName,
		params.FileUpload.OriginalName,
		RelativePathFromUploadMetadata(params.FileUpload),
	)
	if err != nil {
		return nil, err
	}
	if params.RelativePath != "" {
		relativePath, err = workspace.NormalizeRelativePath(params.RelativePath)
		if err != nil {
			return nil, err
		}
	}
	relativePath, err = ApplyTaskUploadScope(relativePath, params.TaskPublicID)
	if err != nil {
		return nil, err
	}

	parentFolder, err := EnsureFolderChain(ctx, db, FolderChainParams{
		OrgID:        params.OrgID,
		ProjectID:    params.ProjectID,
		TaskID:       params.TaskID,
		Uin:          params.Uin,
		ResourceType: types.ProjectFileResourceTypeUserUpload,
		RootPrefix:   "uploads",
		RelativePath: relativePath,
	})
	if err != nil {
		return nil, err
	}

	pf := &types.ProjectFile{
		FilePublicID: params.FileUpload.PublicID,
		OrgID:        params.OrgID,
		ProjectID:    params.ProjectID,
		ResourceID:   params.FileUpload.ID,
		ResourceType: types.ProjectFileResourceTypeUserUpload,
		Uin:          params.Uin,
		NodeType:     types.ProjectFileNodeTypeFile,
		RelativePath: relativePath,
	}
	if parentFolder != nil {
		pf.ParentID = parentFolder.ID
		pf.ParentIDs = append(append([]uint(nil), parentFolder.ParentIDs...), parentFolder.ID)
	}
	if params.TaskID != nil {
		pf.TaskID = *params.TaskID
	}
	if err := infradb.CreateProjectFileVersion(ctx, db, pf); err != nil {
		return nil, err
	}
	return pf, nil
}

// EnsureArtifactFolderChain ensures folder ancestors exist for one artifact path.
func EnsureArtifactFolderChain(
	ctx context.Context,
	db *gorm.DB,
	orgID uint,
	projectID uint,
	taskID *uint,
	uin uint,
	relativePath string,
) (*types.ProjectFile, error) {
	return EnsureFolderChain(ctx, db, FolderChainParams{
		OrgID:        orgID,
		ProjectID:    projectID,
		TaskID:       taskID,
		Uin:          uin,
		ResourceType: types.ProjectFileResourceTypeArtifact,
		RootPrefix:   "artifacts",
		RelativePath: relativePath,
	})
}

// NormalizeFolderRelativePath canonicalizes one folder path with a trailing slash.
func NormalizeFolderRelativePath(relativePath string) (string, error) {
	normalized, err := workspace.NormalizeRelativePath(relativePath)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}
	return normalized, nil
}

// ApplyTaskUploadScope isolates task-scoped uploads under uploads/_task/{taskPublicID}/.
func ApplyTaskUploadScope(relativePath, taskPublicID string) (string, error) {
	taskPublicID = strings.TrimSpace(taskPublicID)
	if taskPublicID == "" {
		return relativePath, nil
	}
	normalized, err := workspace.NormalizeRelativePath(relativePath)
	if err != nil {
		return "", err
	}
	if strings.Contains(normalized, "/"+taskUploadScopeSegment+"/") {
		return normalized, nil
	}
	inner := strings.TrimPrefix(normalized, "uploads/")
	return workspace.NormalizeRelativePath("uploads/" + taskUploadScopeSegment + "/" + taskPublicID + "/" + inner)
}

// ResolveUserUploadRelativePath normalizes one upload path under uploads/.
func ResolveUserUploadRelativePath(displayName, originalName, metadataPath string) (string, error) {
	candidate := strings.TrimSpace(displayName)
	if candidate == "" || !strings.Contains(candidate, "/") {
		candidate = strings.TrimSpace(metadataPath)
	}
	if candidate == "" {
		candidate = strings.TrimSpace(originalName)
	}
	if candidate == "" {
		return "", fmt.Errorf("relative path is required")
	}
	candidate = strings.TrimPrefix(candidate, "/")
	candidate = strings.TrimPrefix(candidate, "uploads/")
	fullPath := "uploads/" + filepath.ToSlash(filepath.Clean(filepath.FromSlash(candidate)))
	return workspace.NormalizeRelativePath(fullPath)
}

// RelativePathFromUploadMetadata reads relative_path from upload metadata.
func RelativePathFromUploadMetadata(fileUpload *types.FileUpload) string {
	if fileUpload == nil || fileUpload.Metadata.Extra == nil {
		return ""
	}
	value, ok := fileUpload.Metadata.Extra["relative_path"]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

// FolderChainParams controls ancestor folder creation for one file path.
type FolderChainParams struct {
	OrgID        uint
	ProjectID    uint
	TaskID       *uint
	Uin          uint
	ResourceType types.ProjectFileResourceType
	RootPrefix   string
	RelativePath string
}

// EnsureFolderChain creates missing folder nodes for one file path.
func EnsureFolderChain(ctx context.Context, db *gorm.DB, params FolderChainParams) (*types.ProjectFile, error) {
	rootPrefix := strings.Trim(strings.TrimSpace(params.RootPrefix), "/")
	if rootPrefix == "" {
		return nil, fmt.Errorf("root prefix is required")
	}
	normalizedPath, err := workspace.NormalizeRelativePath(params.RelativePath)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(normalizedPath, rootPrefix+"/") && normalizedPath != rootPrefix {
		return nil, fmt.Errorf("relative path must be under %s", rootPrefix)
	}

	segments := strings.Split(strings.TrimPrefix(normalizedPath, rootPrefix+"/"), "/")
	if len(segments) == 0 {
		return nil, fmt.Errorf("relative path has no file segment")
	}
	folderSegments := segments[:len(segments)-1]

	rootFolder, err := ensureFolder(ctx, db, ensureFolderParams{
		OrgID:        params.OrgID,
		ProjectID:    params.ProjectID,
		TaskID:       params.TaskID,
		Uin:          params.Uin,
		ResourceType: params.ResourceType,
		RelativePath: rootPrefix + "/",
		Parent:       nil,
	})
	if err != nil {
		return nil, err
	}

	currentParent := rootFolder
	currentPath := rootPrefix + "/"
	for _, segment := range folderSegments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		currentPath = currentPath + segment + "/"
		folder, err := ensureFolder(ctx, db, ensureFolderParams{
			OrgID:        params.OrgID,
			ProjectID:    params.ProjectID,
			TaskID:       params.TaskID,
			Uin:          params.Uin,
			ResourceType: params.ResourceType,
			RelativePath: currentPath,
			Parent:       currentParent,
		})
		if err != nil {
			return nil, err
		}
		currentParent = folder
	}
	return currentParent, nil
}

type ensureFolderParams struct {
	OrgID        uint
	ProjectID    uint
	TaskID       *uint
	Uin          uint
	ResourceType types.ProjectFileResourceType
	RelativePath string
	Parent       *types.ProjectFile
}

func ensureFolder(ctx context.Context, db *gorm.DB, params ensureFolderParams) (*types.ProjectFile, error) {
	folderPath, err := NormalizeFolderRelativePath(params.RelativePath)
	if err != nil {
		return nil, err
	}

	existing, err := infradb.GetProjectFolderByRelativePath(
		ctx,
		db,
		params.OrgID,
		params.ProjectID,
		params.ResourceType,
		folderPath,
	)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	publicID := fmt.Sprintf("folder_%s", snowflake.GenerateIDBase58())
	folder := &types.ProjectFile{
		FilePublicID:        publicID,
		OrgID:               params.OrgID,
		ProjectID:           params.ProjectID,
		ResourceType:        params.ResourceType,
		Uin:                 params.Uin,
		NodeType:            types.ProjectFileNodeTypeFolder,
		RelativePath:        folderPath,
		InitialFilePublicID: publicID,
		VersionNo:           1,
	}
	if params.Parent != nil {
		folder.ParentID = params.Parent.ID
		folder.ParentIDs = append(append([]uint(nil), params.Parent.ParentIDs...), params.Parent.ID)
	}
	if params.TaskID != nil {
		folder.TaskID = *params.TaskID
	}
	if err := infradb.CreateProjectFile(ctx, db, folder); err != nil {
		return nil, err
	}
	return folder, nil
}
