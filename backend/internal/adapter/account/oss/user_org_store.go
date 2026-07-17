//go:build !enterprise

package oss

import (
	"context"

	"gorm.io/gorm"

	infra_db "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

type userOrgStore struct {
	db *gorm.DB
}

func newUserOrgStore(db *gorm.DB) *userOrgStore { return &userOrgStore{db: db} }

func (s *userOrgStore) Get(ctx context.Context, ref UserOrgRef) (*types.UserOrg, error) {
	switch {
	case ref.ID != 0:
		return infra_db.GetUserOrgByID(ctx, s.db, ref.ID)
	case ref.ExternalUin != 0:
		return infra_db.GetUserOrgByExternalUin(ctx, s.db, ref.ExternalUin)
	case ref.Uin != 0 && ref.OrgID != 0:
		return infra_db.GetUserOrgByUinAndOrgID(ctx, s.db, ref.Uin, ref.OrgID)
	case ref.Uin != 0:
		return infra_db.GetUserOrgByUin(ctx, s.db, ref.Uin)
	}
	return nil, nil
}

func (s *userOrgStore) GetByUserID(ctx context.Context, userID uint) (*types.UserOrg, error) {
	return infra_db.GetUserOrgByUserID(ctx, s.db, userID)
}

func (s *userOrgStore) ListByUserID(ctx context.Context, userID uint) ([]*types.UserOrg, error) {
	return infra_db.GetUserOrgsByUserID(ctx, s.db, userID)
}

func (s *userOrgStore) Create(ctx context.Context, uo *types.UserOrg) (*types.UserOrg, error) {
	if err := infra_db.CreateUserOrg(ctx, s.db, uo); err != nil {
		return nil, err
	}
	return uo, nil
}

func (s *userOrgStore) Update(ctx context.Context, uo *types.UserOrg) (*types.UserOrg, error) {
	if err := infra_db.UpdateUserOrg(ctx, s.db, uo); err != nil {
		return nil, err
	}
	return uo, nil
}

func (s *userOrgStore) Delete(ctx context.Context, id uint) error {
	return infra_db.DeleteUserOrg(ctx, s.db, id)
}

func (s *userOrgStore) DeleteMemberDepartments(ctx context.Context, uin uint, orgID uint) error {
	return infra_db.DeleteMemberDepartmentsByUinAndOrgID(ctx, s.db, uin, orgID)
}

func (s *userOrgStore) CountByUserID(ctx context.Context, userID uint) (int64, error) {
	return infra_db.CountUserOrgsByUserID(ctx, s.db, userID)
}

func (s *userOrgStore) ListByUinAndOrgID(ctx context.Context, uin uint, orgID uint) ([]*types.MemberDepartment, error) {
	return infra_db.ListMemberDepartmentsByUinAndOrgID(ctx, s.db, uin, orgID)
}

func (s *userOrgStore) ListByUin(ctx context.Context, uin uint) ([]*types.MemberDepartment, error) {
	return infra_db.ListMemberDepartmentsByUin(ctx, s.db, uin)
}

func (s *userOrgStore) CreateMemberDepartments(ctx context.Context, deps []*types.MemberDepartment) error {
	return infra_db.CreateMemberDepartments(ctx, s.db, deps)
}
