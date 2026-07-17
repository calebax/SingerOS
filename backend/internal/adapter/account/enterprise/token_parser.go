//go:build enterprise

package enterprise

import (
	"context"
	"github.com/insmtx/Leros/backend/pkg/accounterror"
	"strings"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/config"
	localauth "github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

type enterpriseTokenParser struct {
	db           *gorm.DB
	iamCfg       *config.IAMConfig
	iamClient    *iamClient
	workerSecret string
	workerCfg    *config.WorkerAuthConfig
}

// NewTokenParser creates an enterprise TokenParser that verifies
// user tokens via the IAM service and Lework-issued worker tokens.
func NewTokenParser(database *gorm.DB, iamCfg *config.IAMConfig, workerSecret string, workerCfg *config.WorkerAuthConfig) *enterpriseTokenParser {
	return &enterpriseTokenParser{
		db:           database,
		iamCfg:       iamCfg,
		iamClient:    newIAMClient(iamCfg),
		workerSecret: strings.TrimSpace(workerSecret),
		workerCfg:    workerCfg,
	}
}

// ParseUser verifies the token by calling the IAM service and resolves
// the local Uin via the ExternalUin mapping table.
func (p *enterpriseTokenParser) ParseUser(ctx context.Context, tokenStr string) (*types.Caller, error) {
	claims, err := p.iamClient.verifyToken(ctx, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims == nil {
		return &types.Caller{State: types.AuthStateFailed}, nil
	}
	userOrg, err := db.GetUserOrgByExternalUin(ctx, p.db, claims.Uin)
	if err != nil || userOrg == nil {
		return &types.Caller{State: types.AuthStateFailed}, nil
	}
	return &types.Caller{
		Uin:   userOrg.Uin,
		OrgID: userOrg.OrgID,
		Kind:  types.CallerKindUser,
		State: types.AuthStateSucc,
	}, nil
}

func (p *enterpriseTokenParser) ParseWorker(ctx context.Context, tokenStr string) (*types.Caller, error) {
	claims, err := localauth.ParseWorkerToken(tokenStr, p.workerSecret)
	if err != nil {
		return nil, err
	}
	return &types.Caller{
		OrgID:    claims.OrgID,
		WorkerID: claims.WorkerID,
		Kind:     types.CallerKindWorker,
		State:    types.AuthStateSucc,
	}, nil
}

func (p *enterpriseTokenParser) IssueWorker(ctx context.Context, orgID, workerID uint, bootstrapToken string) (string, int64, error) {
	return "", 0, accounterror.ErrNotImplementedEdition
}
