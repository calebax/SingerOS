//go:build enterprise

package enterprise

import (
	"context"
	"github.com/insmtx/Leros/backend/pkg/accounterror"

	"github.com/insmtx/Leros/backend/internal/api/contract"
)

type department struct{}

func NewDepartment() *department {
	return &department{}
}

func (s *department) CreateDepartment(ctx context.Context, req *contract.CreateDepartmentRequest) (*contract.Department, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *department) GetDepartment(ctx context.Context, id uint) (*contract.Department, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *department) UpdateDepartment(ctx context.Context, id uint, req *contract.UpdateDepartmentRequest) (*contract.Department, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *department) DeleteDepartment(ctx context.Context, id uint) error {
	return accounterror.ErrNotImplementedEdition
}

func (s *department) ListDepartments(ctx context.Context, req *contract.ListDepartmentsRequest) (*contract.DepartmentList, error) {
	return nil, accounterror.ErrNotImplementedEdition
}
