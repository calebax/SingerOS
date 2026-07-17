//go:build !enterprise

package account

import (
	"github.com/insmtx/Leros/backend/internal/adapter/account/oss"
	"github.com/insmtx/Leros/backend/internal/api/contract"
)

// NewAuth returns the oss (open-source) AuthService implementation.
func NewAuth(deps Deps) contract.AuthService {
	return oss.NewAuth(deps.DB, deps.JWTSecret, deps.SmsSender, deps.WorkerProvisioning)
}

// NewUser returns the oss (open-source) UserService implementation.
func NewUser(deps Deps) contract.UserService {
	return oss.NewUser(deps.DB)
}

// NewOrg returns the oss (open-source) OrgService implementation.
func NewOrg(deps Deps) contract.OrgService {
	return oss.NewOrg(deps.DB, deps.WorkerProvisioning)
}

// NewDepartment returns the oss (open-source) DepartmentService implementation.
func NewDepartment(deps Deps) contract.DepartmentService {
	return oss.NewDepartment(deps.DB)
}

// NewPermission returns the oss (open-source) PermissionService implementation.
func NewPermission(deps Deps) contract.PermissionService {
	return oss.NewPermission(deps.DB)
}

// NewTokenParser returns the oss (open-source) TokenParser implementation.
func NewTokenParser(deps Deps) TokenParser {
	return oss.NewTokenParser(deps.DB, deps.JWTSecret, deps.WorkerAuth)
}
