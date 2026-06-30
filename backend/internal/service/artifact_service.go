package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/infra/filestore"
	"github.com/insmtx/Leros/backend/types"
	"gorm.io/gorm"
)

type artifactService struct {
	db *gorm.DB
}

func NewArtifactService(db *gorm.DB, _ interface{}) contract.ArtifactService {
	return &artifactService{db: db}
}

func (s *artifactService) ListTaskArtifacts(ctx context.Context, taskPublicID string) ([]contract.Artifact, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(taskPublicID) == "" {
		return nil, errors.New("task_id is required")
	}
	task, err := infradb.GetTaskByPublicID(ctx, s.db, caller.OrgID, taskPublicID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("task not found")
	}
	if err := verifyUserPermission(task.OwnerID, caller.Uin); err != nil {
		return nil, err
	}

	// Query ProjectFile records for this task with resource_type=artifact.
	projectFiles, err := s.listArtifactProjectFilesByTask(ctx, caller.OrgID, task.ID)
	if err != nil {
		return nil, err
	}

	result := make([]contract.Artifact, 0, len(projectFiles))
	for _, pf := range projectFiles {
		fileUpload, err := infradb.GetFileUploadByPublicID(ctx, s.db, caller.OrgID, pf.FilePublicID)
		if err != nil {
			return nil, err
		}
		if fileUpload == nil {
			continue
		}
		result = append(result, convertFileUploadToContractArtifact(fileUpload, pf.CreatedAt))
	}
	return result, nil
}

func (s *artifactService) GetArtifact(ctx context.Context, artifactPublicID string) (*contract.ArtifactDetail, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(artifactPublicID) == "" {
		return nil, errors.New("artifact_id is required")
	}

	fileUpload, projectFile, err := s.resolveArtifactFileUpload(ctx, caller.OrgID, artifactPublicID)
	if err != nil {
		return nil, err
	}
	if fileUpload == nil {
		return nil, errors.New("artifact not found")
	}

	createdAt := fileUpload.CreatedAt
	if projectFile != nil {
		createdAt = projectFile.CreatedAt
	}
	return convertFileUploadToArtifactDetail(fileUpload, createdAt), nil
}

func (s *artifactService) GetArtifactDownload(ctx context.Context, artifactPublicID string) (*contract.ArtifactDownload, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(artifactPublicID) == "" {
		return nil, errors.New("artifact_id is required")
	}

	fileUpload, _, err := s.resolveArtifactFileUpload(ctx, caller.OrgID, artifactPublicID)
	if err != nil {
		return nil, err
	}
	if fileUpload == nil {
		return nil, errors.New("artifact not found")
	}

	storageURI := strings.TrimSpace(fileUpload.StorageURI)
	if storageURI == "" {
		return nil, errors.New("artifact has no storage uri")
	}

	bucket, key, err := filestore.ParseStorageURI(storageURI)
	if err != nil {
		return nil, fmt.Errorf("parse artifact storage uri: %w", err)
	}

	st := filestore.GetStorage()
	obj, err := st.GetObject(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("read artifact from storage: %w", err)
	}

	return &contract.ArtifactDownload{
		FileName: artifactDownloadNameFromUpload(fileUpload),
		MimeType: fileUpload.MimeType,
		Size:     fileUpload.FileSize,
		Reader:   obj.Body,
	}, nil
}

// resolveArtifactFileUpload treats artifactPublicID as a FileUpload.PublicID.
func (s *artifactService) resolveArtifactFileUpload(ctx context.Context, orgID uint, artifactPublicID string) (*types.FileUpload, *types.ProjectFile, error) {
	fileUpload, err := infradb.GetFileUploadByPublicID(ctx, s.db, orgID, artifactPublicID)
	if err != nil {
		return nil, nil, err
	}
	if fileUpload == nil {
		return nil, nil, nil
	}
	projectFile, err := infradb.GetProjectFileByFilePublicID(ctx, s.db, orgID, artifactPublicID)
	if err != nil {
		return nil, nil, err
	}
	return fileUpload, projectFile, nil
}

// listArtifactProjectFilesByTask returns ProjectFile records for a task with resource_type=artifact.
func (s *artifactService) listArtifactProjectFilesByTask(ctx context.Context, orgID, taskID uint) ([]types.ProjectFile, error) {
	var files []types.ProjectFile
	err := s.db.WithContext(ctx).Model(&types.ProjectFile{}).
		Where("org_id = ? AND task_id = ? AND resource_type = ?", orgID, taskID, types.ProjectFileResourceTypeArtifact).
		Order("created_at DESC").
		Find(&files).Error
	return files, err
}

func convertFileUploadToContractArtifact(fileUpload *types.FileUpload, createdAt time.Time) contract.Artifact {
	return contract.Artifact{
		ArtifactID:   fileUpload.PublicID,
		Title:        fileUpload.Filename,
		Filename:     fileUpload.Filename,
		Description:  "",
		ArtifactType: string(types.ArtifactTypeFile),
		MimeType:     fileUpload.MimeType,
		FileSize:     fileUpload.FileSize,
		Sha256:       fileUpload.Sha256,
		CreatedAt:    createdAt,
	}
}

func convertFileUploadToArtifactDetail(fileUpload *types.FileUpload, createdAt time.Time) *contract.ArtifactDetail {
	return &contract.ArtifactDetail{
		Artifact:     convertFileUploadToContractArtifact(fileUpload, createdAt),
		RelativePath: "",
		FilePublicID: fileUpload.PublicID,
		Source:       string(types.ArtifactSourceAgentDeclared),
		ExportFormat: "",
		Version:      1,
		Status:       string(types.ArtifactStatusCompleted),
	}
}

func artifactDownloadNameFromUpload(fileUpload *types.FileUpload) string {
	if fileUpload == nil {
		return ""
	}
	if strings.TrimSpace(fileUpload.Filename) != "" {
		return strings.TrimSpace(fileUpload.Filename)
	}
	return filepath.Base(strings.TrimSpace(fileUpload.StorageURI))
}

var _ contract.ArtifactService = (*artifactService)(nil)
