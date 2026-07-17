//go:build !enterprise

package adapter

import (
	"github.com/insmtx/Leros/backend/internal/adapter/account"
)

type ossEdition struct {
	auth        account.AuthProvider
	user        account.UserRepository
	org         account.OrgRepository
	department  account.DepartmentRepository
	permission  account.PermissionProvider
	tokenParser account.TokenParser
}

// NewEdition returns the oss (open-source) edition. Selected at build time
// when no -tags enterprise is passed.
func NewEdition(cfg Config) Edition {
	deps := cfg.ToDeps()
	return &ossEdition{
		auth:        account.NewAuth(deps),
		user:        account.NewUser(deps),
		org:         account.NewOrg(deps),
		department:  account.NewDepartment(deps),
		permission:  account.NewPermission(deps),
		tokenParser: account.NewTokenParser(deps),
	}
}

func (e *ossEdition) Auth() account.AuthProvider             { return e.auth }
func (e *ossEdition) User() account.UserRepository           { return e.user }
func (e *ossEdition) Org() account.OrgRepository             { return e.org }
func (e *ossEdition) Department() account.DepartmentRepository { return e.department }
func (e *ossEdition) Permission() account.PermissionProvider { return e.permission }
func (e *ossEdition) TokenParser() account.TokenParser       { return e.tokenParser }
