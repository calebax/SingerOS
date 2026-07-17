//go:build enterprise

package enterprise

import (
	"context"
	"github.com/insmtx/Leros/backend/pkg/accounterror"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/config"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/service"
)

type auth struct {
	db           *gorm.DB
	iamCfg       *config.IAMConfig
	client       *iamClient
	provisioning *service.WorkerProvisioningService
}

// NewAuth creates an enterprise auth adapter that delegates
// authentication to the IAM service.
func NewAuth(db *gorm.DB, iamCfg *config.IAMConfig, provisioning *service.WorkerProvisioningService) *auth {
	return &auth{
		db:           db,
		iamCfg:       iamCfg,
		client:       newIAMClient(iamCfg),
		provisioning: provisioning,
	}
}

func (s *auth) RegisterByEmail(ctx context.Context, req *contract.RegisterByEmailRequest) (*contract.AuthTokenResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *auth) LoginByEmail(ctx context.Context, req *contract.LoginByEmailRequest) (*contract.AuthTokenResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

// SendPhoneLoginCode in enterprise mode delegates the full verification-code
// flow (code generation, SMS delivery, storage, verification, JWT issuance)
// to the IAM service via iamClient. Not yet implemented.
//
// Planned: call IAM POST /account.SendPhoneCode with {Phone}, return IAM's
// {Phone, ExpiresIn, ResendAfter} verbatim; Lework enterprise does NOT hold
// the code, persist it, or call infra/sms.
//
// See docs/design/account-convergence-tech-design.md §5.3, §6.7.
func (s *auth) SendPhoneLoginCode(ctx context.Context, req *contract.SendPhoneLoginCodeRequest) (*contract.SendPhoneLoginCodeResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

// LoginByPhoneCode in enterprise mode delegates the entire login flow
// (code comparison, mark-used, auto-register, JWT issuance) to the IAM
// service via iamClient. Not yet implemented.
//
// Planned: call IAM POST /account.LoginByPhoneCode with {Phone, Code, OrgID},
// return IAM's AuthTokenResponse verbatim.
//
// See docs/design/account-convergence-tech-design.md §5.3, §6.7.
func (s *auth) LoginByPhoneCode(ctx context.Context, req *contract.LoginByPhoneCodeRequest) (*contract.AuthTokenResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *auth) RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.AuthTokenResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *auth) SwitchOrganization(ctx context.Context, req *contract.SwitchOrganizationRequest) (*contract.AuthTokenResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *auth) CreateOrganization(ctx context.Context, req *contract.CreateOrganizationRequest) (*contract.AuthTokenResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}

func (s *auth) AuthSession(ctx context.Context) (*contract.AuthSessionResponse, error) {
	return nil, accounterror.ErrNotImplementedEdition
}
