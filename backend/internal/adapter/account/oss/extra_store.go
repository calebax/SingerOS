//go:build !enterprise

package oss

import (
	"context"

	"gorm.io/gorm"

	infra_db "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

type extraStore struct {
	db *gorm.DB
}

func newExtraStore(db *gorm.DB) *extraStore { return &extraStore{db: db} }

func (s *extraStore) GetProjectByPublicID(ctx context.Context, orgID uint, publicID string) (*types.Project, error) {
	return infra_db.GetProjectByPublicID(ctx, s.db, orgID, publicID)
}

func (s *extraStore) GetProjectFileByFilePublicID(ctx context.Context, projectID uint, filePublicID string) (*types.ProjectFile, error) {
	return infra_db.GetProjectFileByFilePublicID(ctx, s.db, projectID, filePublicID)
}

func (s *extraStore) GetTaskByPublicID(ctx context.Context, orgID uint, publicID string) (*types.Task, error) {
	return infra_db.GetTaskByPublicID(ctx, s.db, orgID, publicID)
}
