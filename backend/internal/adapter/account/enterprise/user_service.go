//go:build enterprise

package enterprise

import (
	"context"
	"github.com/insmtx/Leros/backend/pkg/accounterror"

	"github.com/insmtx/Leros/backend/internal/api/contract"
)

type user struct{}

func NewUser() *user {
	return &user{}
}

func (s *user) CreateUser(ctx context.Context, req *contract.CreateUserRequest) (*contract.UserInfo, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *user) GetUser(ctx context.Context, publicID string, githubLogin string) (*contract.UserInfo, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *user) UpdateUser(ctx context.Context, publicID string, req *contract.UpdateUserRequest) (*contract.UserInfo, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *user) DeleteUser(ctx context.Context, publicID string) error {
	return accounterror.ErrNotImplementedEdition
}

func (s *user) ListUsers(ctx context.Context, req *contract.ListUsersRequest) (*contract.UserList, error) {
	return nil, accounterror.ErrNotImplementedEdition
}
