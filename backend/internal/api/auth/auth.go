package auth

import (
	"context"

	"github.com/insmtx/SingerOS/backend/internal/api/dto"
)

// ProviderAuthResolver resolves one provider-specific authorization path.
type ProviderAuthResolver interface {
	ResolveAuthorization(ctx context.Context, req *dto.ResolveAuthorizationRequest) (*dto.ResolvedAuthorization, bool, error)
}

// AuthorizationProvider 定义 provider 授权接入所需接口。
type AuthorizationProvider interface {
	ProviderCode() string
	BuildAuthorizationURL(req *dto.StartAuthorizationRequest, state *dto.OAuthState) (string, error)
	CompleteAuthorization(req *dto.CompleteAuthorizationRequest) (*dto.AuthorizationResult, error)
}
