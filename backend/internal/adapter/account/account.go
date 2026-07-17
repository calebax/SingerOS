package account

import (
	"context"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

// AuthProvider defines authentication capabilities. Oss and enterprise
// implementations share this contract; the concrete implementation is
// selected at build time via //go:build tags in the adapter factory.
type AuthProvider interface {
	RegisterByEmail(ctx context.Context, req *contract.RegisterByEmailRequest) (*contract.AuthTokenResponse, error)
	LoginByEmail(ctx context.Context, req *contract.LoginByEmailRequest) (*contract.AuthTokenResponse, error)
	SendPhoneLoginCode(ctx context.Context, req *contract.SendPhoneLoginCodeRequest) (*contract.SendPhoneLoginCodeResponse, error)
	LoginByPhoneCode(ctx context.Context, req *contract.LoginByPhoneCodeRequest) (*contract.AuthTokenResponse, error)
	RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.AuthTokenResponse, error)
	SwitchOrganization(ctx context.Context, req *contract.SwitchOrganizationRequest) (*contract.AuthTokenResponse, error)
	CreateOrganization(ctx context.Context, req *contract.CreateOrganizationRequest) (*contract.AuthTokenResponse, error)
	AuthSession(ctx context.Context) (*contract.AuthSessionResponse, error)
}

// UserRepository defines user data management capabilities. Both oss
// (database-backed) and enterprise (HTTP-backed) implementations
// realize this interface.
type UserRepository interface {
	CreateUser(ctx context.Context, req *contract.CreateUserRequest) (*contract.UserInfo, error)
	GetUser(ctx context.Context, publicID string, githubLogin string) (*contract.UserInfo, error)
	UpdateUser(ctx context.Context, publicID string, req *contract.UpdateUserRequest) (*contract.UserInfo, error)
	DeleteUser(ctx context.Context, publicID string) error
	ListUsers(ctx context.Context, req *contract.ListUsersRequest) (*contract.UserList, error)
}

// OrgRepository defines organization management capabilities shared
// across oss and enterprise editions.
type OrgRepository interface {
	CreateOrg(ctx context.Context, req *contract.CreateOrgRequest) (*contract.Org, error)
	GetOrg(ctx context.Context, publicID string, code string) (*contract.Org, error)
	UpdateOrg(ctx context.Context, publicID string, req *contract.UpdateOrgRequest) (*contract.Org, error)
	DeleteOrg(ctx context.Context, publicID string) error
	ListOrgs(ctx context.Context, req *contract.ListOrgsRequest) (*contract.OrgList, error)

	CreateOrgMember(ctx context.Context, req *contract.CreateOrgMemberRequest) (*contract.OrgMember, error)
	GetOrgMember(ctx context.Context, id uint, uin uint) (*contract.OrgMember, error)
	UpdateOrgMember(ctx context.Context, id uint, req *contract.UpdateOrgMemberRequest) (*contract.OrgMember, error)
	ListOrgMembers(ctx context.Context, req *contract.ListOrgMembersRequest) (*contract.OrgMemberList, error)
}

// DepartmentRepository defines department management capabilities.
type DepartmentRepository interface {
	CreateDepartment(ctx context.Context, req *contract.CreateDepartmentRequest) (*contract.Department, error)
	GetDepartment(ctx context.Context, id uint) (*contract.Department, error)
	UpdateDepartment(ctx context.Context, id uint, req *contract.UpdateDepartmentRequest) (*contract.Department, error)
	DeleteDepartment(ctx context.Context, id uint) error
	ListDepartments(ctx context.Context, req *contract.ListDepartmentsRequest) (*contract.DepartmentList, error)
}

// PermissionProvider defines core permission evaluation capabilities.
type PermissionProvider interface {
	Can(ctx context.Context, caller types.PermissionCaller, action types.Action,
		ref types.ResourceRef, input *types.MemberInput) (types.PermissionDecision, error)
	Explain(ctx context.Context, caller types.PermissionCaller, action types.Action,
		ref types.ResourceRef, input *types.MemberInput) (types.PermissionExplainDecision, error)
	BatchCan(ctx context.Context, caller types.PermissionCaller, actions []types.Action,
		refs []types.ResourceRef, input *types.MemberInput) ([]types.PermissionDecision, error)
	BatchCheckByPublicID(ctx context.Context, caller types.PermissionCaller,
		items []contract.BatchCheckPermissionItem) ([]contract.BatchCheckPermissionResult, error)
	ResolveEffectiveRole(ctx context.Context, caller types.PermissionCaller, resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error)
}

// TokenParser abstracts JWT parsing and worker token issuance so that
// the middleware remains agnostic of whether authentication is handled
// locally (oss) or delegated to IAM (enterprise).
type TokenParser interface {
	ParseUser(ctx context.Context, tokenStr string) (*types.Caller, error)
	ParseWorker(ctx context.Context, tokenStr string) (*types.Caller, error)
	IssueWorker(ctx context.Context, orgID, workerID uint, bootstrapToken string) (token string, expiredAt int64, err error)
}
