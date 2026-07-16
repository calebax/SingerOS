//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/testutil"
	"github.com/insmtx/Leros/backend/types"
)

func TestAuthServiceRegisterByEmail_Integration(t *testing.T) {
	db := testutil.Setup(t)
	svc := NewAuthService(db, "test-secret", nil)

	registered, err := svc.RegisterByEmail(context.Background(), &contract.RegisterByEmailRequest{
		Email:           "integration.test@example.com",
		Password:        "Password123",
		ConfirmPassword: "Password123",
		Name:            "Integration Test",
	})
	if err != nil {
		t.Fatalf("RegisterByEmail failed: %v", err)
	}
	if registered.JwtToken == "" {
		t.Fatal("expected jwt token")
	}
	if registered.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}
	if registered.Uin == 0 {
		t.Fatal("expected uin")
	}
	if registered.Org.ID != types.SystemOrgID {
		t.Fatalf("expected org ID %d, got %d", types.SystemOrgID, registered.Org.ID)
	}
}

func TestAuthServiceLoginByEmail_Integration(t *testing.T) {
	db := testutil.Setup(t)
	svc := NewAuthService(db, "test-secret", nil)
	ctx := context.Background()

	_, err := svc.RegisterByEmail(ctx, &contract.RegisterByEmailRequest{
		Email:           "login.test@example.com",
		Password:        "Password123",
		ConfirmPassword: "Password123",
		Name:            "Login Test",
	})
	if err != nil {
		t.Fatalf("RegisterByEmail failed: %v", err)
	}

	loginResp, err := svc.LoginByEmail(ctx, &contract.LoginByEmailRequest{
		Email:    "login.test@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("LoginByEmail failed: %v", err)
	}
	if loginResp.JwtToken == "" {
		t.Fatal("expected jwt token")
	}
	if loginResp.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}
}

func TestAuthServiceRegisterByEmail_DuplicateEmail_Integration(t *testing.T) {
	db := testutil.Setup(t)
	svc := NewAuthService(db, "test-secret", nil)
	ctx := context.Background()

	_, err := svc.RegisterByEmail(ctx, &contract.RegisterByEmailRequest{
		Email:           "duplicate@example.com",
		Password:        "Password123",
		ConfirmPassword: "Password123",
		Name:            "Dup User",
	})
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, err = svc.RegisterByEmail(ctx, &contract.RegisterByEmailRequest{
		Email:           "duplicate@example.com",
		Password:        "Password456",
		ConfirmPassword: "Password456",
		Name:            "Dup User 2",
	})
	if err == nil {
		t.Fatal("expected error for duplicate email registration")
	}
}
