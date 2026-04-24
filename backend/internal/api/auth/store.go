package auth

import (
	"context"

	"github.com/insmtx/SingerOS/backend/internal/api/dto"
)

// Store 定义授权账户的存储接口。
type Store interface {
	SaveOAuthState(ctx context.Context, state *dto.OAuthState) error
	ConsumeOAuthState(ctx context.Context, provider, state string) (*dto.OAuthState, error)

	UpsertAuthorizedAccount(ctx context.Context, account *dto.AuthorizedAccount, credential *dto.AccountCredential) error
	GetAuthorizedAccount(ctx context.Context, accountID string) (*dto.AuthorizedAccount, error)
	ListUserAccounts(ctx context.Context, userID, provider string) ([]*dto.AuthorizedAccount, error)

	GetCredential(ctx context.Context, accountID string) (*dto.AccountCredential, error)

	SetDefaultAccount(ctx context.Context, binding *dto.UserProviderBinding) error
	GetDefaultAccount(ctx context.Context, userID, provider string) (*dto.AuthorizedAccount, error)
}
