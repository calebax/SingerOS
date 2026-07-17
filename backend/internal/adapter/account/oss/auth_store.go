//go:build !enterprise

package oss

import (
	"context"
	"time"

	"gorm.io/gorm"

	infra_db "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

type authStore struct {
	db *gorm.DB
}

func newAuthStore(db *gorm.DB) *authStore { return &authStore{db: db} }

func (s *authStore) CreatePhoneCode(ctx context.Context, code *types.AuthPhoneVerificationCode) error {
	return infra_db.CreateAuthPhoneVerificationCode(ctx, s.db, code)
}

func (s *authStore) GetActivePhoneCode(ctx context.Context, phone string, now time.Time) (*types.AuthPhoneVerificationCode, error) {
	return infra_db.GetActiveAuthPhoneVerificationCode(ctx, s.db, phone, now)
}

func (s *authStore) DeleteExpiredPhoneCodes(ctx context.Context, now time.Time) error {
	return infra_db.DeleteExpiredAuthPhoneVerificationCodes(ctx, s.db, now)
}

func (s *authStore) CreateRefreshToken(ctx context.Context, token *types.AuthRefreshToken) error {
	return infra_db.CreateAuthRefreshToken(ctx, s.db, token)
}

func (s *authStore) GetActiveRefreshToken(ctx context.Context, tokenHash string, now time.Time) (*types.AuthRefreshToken, error) {
	return infra_db.GetActiveAuthRefreshToken(ctx, s.db, tokenHash, now)
}

func (s *authStore) DeleteExpiredRefreshTokens(ctx context.Context, now time.Time) error {
	return infra_db.DeleteExpiredAuthRefreshTokens(ctx, s.db, now)
}

func (s *authStore) GetLoginAttempt(ctx context.Context, key string) (*types.AuthLoginAttempt, error) {
	return infra_db.GetAuthLoginAttempt(ctx, s.db, key)
}

func (s *authStore) DeleteLoginAttempt(ctx context.Context, key string) error {
	return infra_db.DeleteAuthLoginAttempt(ctx, s.db, key)
}

func (s *authStore) DeleteExpiredLoginAttempts(ctx context.Context, now time.Time) error {
	return infra_db.DeleteExpiredAuthLoginAttempts(ctx, s.db, now)
}

func (s *authStore) SaveLoginAttempt(ctx context.Context, attempts *types.AuthLoginAttempt) error {
	return infra_db.SaveAuthLoginAttempt(ctx, s.db, attempts)
}

func (s *authStore) IsUniqueConstraintError(err error) bool {
	return infra_db.IsUniqueConstraintError(err)
}
