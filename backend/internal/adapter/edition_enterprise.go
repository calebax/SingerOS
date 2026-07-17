//go:build enterprise

package adapter

import (
	"github.com/insmtx/Leros/backend/internal/adapter/account"
)

type enterpriseEdition struct {
	auth        account.AuthProvider
	user        account.UserRepository
	org         account.OrgRepository
	department  account.DepartmentRepository
	permission  account.PermissionProvider
	tokenParser account.TokenParser
}

// NewEdition returns the enterprise edition. Selected at build time when
// -tags enterprise is passed.
func NewEdition(cfg Config) Edition {
	deps := cfg.ToDeps()
	return &enterpriseEdition{
		auth:        account.NewAuth(deps),
		user:        account.NewUser(deps),
		org:         account.NewOrg(deps),
		department:  account.NewDepartment(deps),
		permission:  account.NewPermission(deps),
		tokenParser: account.NewTokenParser(deps),
	}
}

func (e *enterpriseEdition) Auth() account.AuthProvider             { return e.auth }
func (e *enterpriseEdition) User() account.UserRepository           { return e.user }
func (e *enterpriseEdition) Org() account.OrgRepository             { return e.org }
func (e *enterpriseEdition) Department() account.DepartmentRepository { return e.department }
func (e *enterpriseEdition) Permission() account.PermissionProvider { return e.permission }
func (e *enterpriseEdition) TokenParser() account.TokenParser       { return e.tokenParser }
