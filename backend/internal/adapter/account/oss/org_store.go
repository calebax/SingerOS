//go:build !enterprise

package oss

import (
	"context"

	"gorm.io/gorm"

	infra_db "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

type orgStore struct {
	db *gorm.DB
}

func newOrgStore(db *gorm.DB) *orgStore { return &orgStore{db: db} }

func (s *orgStore) Get(ctx context.Context, ref OrgRef) (*types.Organization, error) {
	switch {
	case ref.ID != 0:
		return infra_db.GetOrgByID(ctx, s.db, ref.ID)
	case ref.PublicID != "":
		return infra_db.GetOrgByPublicID(ctx, s.db, ref.PublicID)
	case ref.Code != "":
		return infra_db.GetOrgByCode(ctx, s.db, ref.Code)
	}
	return nil, nil
}

func (s *orgStore) GetByIDs(ctx context.Context, ids []uint) ([]*types.Organization, error) {
	return infra_db.GetOrgsByIDs(ctx, s.db, ids)
}

func (s *orgStore) Create(ctx context.Context, o *types.Organization) (*types.Organization, error) {
	if err := infra_db.CreateOrg(ctx, s.db, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *orgStore) Update(ctx context.Context, o *types.Organization) (*types.Organization, error) {
	if err := infra_db.UpdateOrg(ctx, s.db, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *orgStore) Delete(ctx context.Context, id uint) error {
	return infra_db.DeleteOrg(ctx, s.db, id)
}

func (s *orgStore) IsUniqueConstraintError(err error) bool {
	return infra_db.IsUniqueConstraintError(err)
}
