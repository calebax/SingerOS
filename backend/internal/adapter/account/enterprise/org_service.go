//go:build enterprise

package enterprise

import (
	"context"
	"github.com/insmtx/Leros/backend/pkg/accounterror"

	"github.com/insmtx/Leros/backend/internal/api/contract"
)

type org struct{}

func NewOrg() *org {
	return &org{}
}

func (s *org) CreateOrg(ctx context.Context, req *contract.CreateOrgRequest) (*contract.Org, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *org) GetOrg(ctx context.Context, publicID string, code string) (*contract.Org, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *org) UpdateOrg(ctx context.Context, publicID string, req *contract.UpdateOrgRequest) (*contract.Org, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *org) DeleteOrg(ctx context.Context, publicID string) error {
	return accounterror.ErrNotImplementedEdition
}

func (s *org) ListOrgs(ctx context.Context, req *contract.ListOrgsRequest) (*contract.OrgList, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *org) CreateOrgMember(ctx context.Context, req *contract.CreateOrgMemberRequest) (*contract.OrgMember, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *org) GetOrgMember(ctx context.Context, id uint, uin uint) (*contract.OrgMember, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *org) UpdateOrgMember(ctx context.Context, id uint, req *contract.UpdateOrgMemberRequest) (*contract.OrgMember, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *org) ListOrgMembers(ctx context.Context, req *contract.ListOrgMembersRequest) (*contract.OrgMemberList, error) {
	return nil, accounterror.ErrNotImplementedEdition
}
