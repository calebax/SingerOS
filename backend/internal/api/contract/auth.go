package contract

import (
	"context"

	"github.com/insmtx/Leros/backend/types"
)

type AuthService interface {
	RegisterByEmail(ctx context.Context, req *RegisterByEmailRequest) (*AuthTokenResponse, error)
	LoginByEmail(ctx context.Context, req *LoginByEmailRequest) (*AuthTokenResponse, error)
	SendPhoneLoginCode(ctx context.Context, req *SendPhoneLoginCodeRequest) (*SendPhoneLoginCodeResponse, error)
	LoginByPhoneCode(ctx context.Context, req *LoginByPhoneCodeRequest) (*AuthTokenResponse, error)
	RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*AuthTokenResponse, error)
	SwitchOrganization(ctx context.Context, req *SwitchOrganizationRequest) (*AuthTokenResponse, error)
	CreateOrganization(ctx context.Context, req *CreateOrganizationRequest) (*AuthTokenResponse, error)
	AuthSession(ctx context.Context) (*AuthSessionResponse, error)
}

type OrgService interface {
	CreateOrg(ctx context.Context, req *CreateOrgRequest) (*Org, error)
	GetOrg(ctx context.Context, publicID string, code string) (*Org, error)
	UpdateOrg(ctx context.Context, publicID string, req *UpdateOrgRequest) (*Org, error)
	DeleteOrg(ctx context.Context, publicID string) error
	ListOrgs(ctx context.Context, req *ListOrgsRequest) (*OrgList, error)

	CreateOrgMember(ctx context.Context, req *CreateOrgMemberRequest) (*OrgMember, error)
	GetOrgMember(ctx context.Context, id uint, uin uint) (*OrgMember, error)
	UpdateOrgMember(ctx context.Context, id uint, req *UpdateOrgMemberRequest) (*OrgMember, error)
	ListOrgMembers(ctx context.Context, req *ListOrgMembersRequest) (*OrgMemberList, error)
}

type DepartmentService interface {
	CreateDepartment(ctx context.Context, req *CreateDepartmentRequest) (*Department, error)
	GetDepartment(ctx context.Context, id uint) (*Department, error)
	UpdateDepartment(ctx context.Context, id uint, req *UpdateDepartmentRequest) (*Department, error)
	DeleteDepartment(ctx context.Context, id uint) error
	ListDepartments(ctx context.Context, req *ListDepartmentsRequest) (*DepartmentList, error)
}

type PermissionService interface {
	Can(ctx context.Context, caller types.PermissionCaller, action types.Action,
		ref types.ResourceRef, input *types.MemberInput) (types.PermissionDecision, error)
	Explain(ctx context.Context, caller types.PermissionCaller, action types.Action,
		ref types.ResourceRef, input *types.MemberInput) (types.PermissionExplainDecision, error)
	BatchCan(ctx context.Context, caller types.PermissionCaller, actions []types.Action,
		refs []types.ResourceRef, input *types.MemberInput) ([]types.PermissionDecision, error)
	BatchCheckByPublicID(ctx context.Context, caller types.PermissionCaller,
		items []BatchCheckPermissionItem) ([]BatchCheckPermissionResult, error)
	ResolveEffectiveRole(ctx context.Context, caller types.PermissionCaller,
		resource *types.Resource) (types.ResourceRole, *types.ResourceBinding, *types.Resource, error)
}
