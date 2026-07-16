package projectfile

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/consts"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/workspace"
	"github.com/insmtx/Leros/backend/types"
)

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

// BindUserUploadParams links one uploaded file into a project.
type BindUserUploadParams struct {
	OrgID        uint
	ProjectID    uint
	TaskID       *uint
	TaskPublicID string
	Uin          uint
	FileUpload   *types.FileUpload
	DisplayName  string
	RelativePath string
}

// BindUserUploadToProject creates a ProjectFile record for one upload.
func BindUserUploadToProject(ctx context.Context, db *gorm.DB, params BindUserUploadParams) (*types.ProjectFile, error) {
	if params.FileUpload == nil {
		return nil, fmt.Errorf("file upload is required")
	}

	relativePath := strings.TrimSpace(params.RelativePath)
	if relativePath == "" {
		relativePath = strings.TrimSpace(params.DisplayName)
	}
	if relativePath == "" {
		relativePath = strings.TrimSpace(params.FileUpload.OriginalName)
	}
	if relativePath == "" {
		return nil, fmt.Errorf("relative path is required")
	}
	relativePath, err := workspace.NormalizeRelativePath(consts.RepoDirUploads + "/" + relativePath)
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
		RelativePath: relativePath,
	}
	if params.TaskID != nil {
		pf.TaskID = *params.TaskID
	}
	if err := infradb.CreateProjectFile(ctx, db, pf); err != nil {
		return nil, err
	}
	return pf, nil
}
