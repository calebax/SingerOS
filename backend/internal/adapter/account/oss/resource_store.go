//go:build !enterprise

package oss

import (
	"context"

	"gorm.io/gorm"

	infra_db "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

type resourceStore struct {
	db *gorm.DB
}

func newResourceStore(db *gorm.DB) *resourceStore { return &resourceStore{db: db} }

func (s *resourceStore) GetByID(ctx context.Context, id uint) (*types.Resource, error) {
	return infra_db.GetResourceByID(ctx, s.db, id)
}

func (s *resourceStore) GetByBizID(ctx context.Context, orgID uint, resourceType types.ResourceType, bizID uint) (*types.Resource, error) {
	return infra_db.GetResourceByBizID(ctx, s.db, orgID, resourceType, bizID)
}

func (s *resourceStore) Create(ctx context.Context, r *types.Resource) (*types.Resource, error) {
	if err := infra_db.CreateResource(ctx, s.db, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *resourceStore) GetBindingByUin(ctx context.Context, uin uint, resourceID uint) (*types.ResourceBinding, error) {
	return infra_db.GetResourceBindingByUin(ctx, s.db, uin, resourceID)
}

func (s *resourceStore) GetBindingByAssistantID(ctx context.Context, resourceID uint, assistantID uint) (*types.ResourceBinding, error) {
	return infra_db.GetResourceBindingByAssistantID(ctx, s.db, resourceID, assistantID)
}

func (s *resourceStore) CreateBinding(ctx context.Context, b *types.ResourceBinding) (*types.ResourceBinding, error) {
	if err := infra_db.CreateResourceBinding(ctx, s.db, b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *resourceStore) ListBindingsByResourceID(ctx context.Context, resourceID uint) ([]*types.ResourceBinding, error) {
	return infra_db.ListResourceBindingsByResourceID(ctx, s.db, resourceID)
}

func (s *resourceStore) CountBindingsByRole(ctx context.Context, resourceID uint, role types.ResourceRole) (int64, error) {
	return infra_db.CountResourceBindingsByRole(ctx, s.db, resourceID, role)
}
