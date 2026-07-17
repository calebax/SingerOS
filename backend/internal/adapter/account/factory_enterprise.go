//go:build enterprise

package account

import (
	"github.com/insmtx/Leros/backend/internal/adapter/account/enterprise"
	"github.com/insmtx/Leros/backend/internal/api/contract"
)

// NewAuth returns the enterprise AuthService implementation that
// delegates authentication to the IAM service.
func NewAuth(deps Deps) contract.AuthService {
	return enterprise.NewAuth(deps.DB, deps.IAM, deps.WorkerProvisioning)
}

// NewUser returns the enterprise UserService implementation that
// delegates user management to the IAM service.
func NewUser(deps Deps) contract.UserService {
	return enterprise.NewUser()
}

// NewOrg returns the enterprise OrgService implementation that
// delegates org management to the IAM service.
func NewOrg(deps Deps) contract.OrgService {
	return enterprise.NewOrg()
}

// NewDepartment returns the enterprise DepartmentService implementation that
// delegates department management to the IAM service.
func NewDepartment(deps Deps) contract.DepartmentService {
	return enterprise.NewDepartment()
}

// NewPermission returns the enterprise PermissionService implementation that
// delegates permission evaluation to the IAM service.
func NewPermission(deps Deps) contract.PermissionService {
	return enterprise.NewPermission()
}

// NewTokenParser returns the enterprise TokenParser implementation that
// verifies user tokens via the IAM service and Lework-issued worker tokens.
func NewTokenParser(deps Deps) TokenParser {
	return enterprise.NewTokenParser(deps.DB, deps.IAM, deps.JWTSecret, deps.WorkerAuth)
}
