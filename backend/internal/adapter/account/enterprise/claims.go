//go:build enterprise

package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// iamTokenClaims carries verified user identity returned by the IAM service
// after token verification.
type iamTokenClaims struct {
	Uin    uint `json:"uin"`
	UserID uint `json:"user_id"`
}

// verifyToken sends the user token to the IAM service for verification
// and returns the decoded identity claims.
func (c *iamClient) verifyToken(ctx context.Context, tokenStr string) (*iamTokenClaims, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "account.VerifyToken")
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verify token: http %d", resp.StatusCode)
	}
	var claims iamTokenClaims
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, fmt.Errorf("decode verify token response: %w", err)
	}
	if claims.Uin == 0 {
		return nil, fmt.Errorf("verify token: invalid uin in response")
	}
	return &claims, nil
}
