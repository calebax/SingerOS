package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// HTTPClient handles all REST API calls to QQ Bot servers.
type HTTPClient struct {
	client      *http.Client
	appID       string
	appSecret   string

	mu         sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// NewHTTPClient creates a new QQ Bot HTTP client.
func NewHTTPClient(appID, appSecret string) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		appID:     appID,
		appSecret: appSecret,
	}
}

// GetToken returns the current access token, refreshing if necessary.
func (c *HTTPClient) GetToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		return c.accessToken, nil
	}

	reqBody := TokenRequest{
		AppID:        c.appID,
		ClientSecret: c.appSecret,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenBaseURL+"/app/getAppAccessToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get token: status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(int(tokenResp.ExpiresIn)-60) * time.Second) // 60s buffer

	return c.accessToken, nil
}

// InvalidateToken forces a token refresh on the next request.
func (c *HTTPClient) InvalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
}

// GetGateway fetches the WebSocket gateway URL.
func (c *HTTPClient) GetGateway(ctx context.Context) (*GatewayResponse, error) {
	var resp GatewayResponse
	if err := c.apiGet(ctx, "/gateway", &resp); err != nil {
		return nil, fmt.Errorf("get gateway: %w", err)
	}
	return &resp, nil
}

// SendC2CMessage sends a message to a private chat user.
func (c *HTTPClient) SendC2CMessage(ctx context.Context, openID string, msg any) (*SendMessageResponse, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	var resp SendMessageResponse
	path := fmt.Sprintf("/v2/users/%s/messages", openID)
	if err := c.apiPost(ctx, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendGroupMessage sends a message to a group chat.
func (c *HTTPClient) SendGroupMessage(ctx context.Context, groupOpenID string, msg any) (*SendMessageResponse, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	var resp SendMessageResponse
	path := fmt.Sprintf("/v2/groups/%s/messages", groupOpenID)
	if err := c.apiPost(ctx, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendChannelMessage sends a message to a guild channel.
func (c *HTTPClient) SendChannelMessage(ctx context.Context, channelID string, msg any) (*SendMessageResponse, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	var resp SendMessageResponse
	path := fmt.Sprintf("/channels/%s/messages", channelID)
	if err := c.apiPost(ctx, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendDMMessage sends a message to a guild DM.
func (c *HTTPClient) SendDMMessage(ctx context.Context, guildID string, msg any) (*SendMessageResponse, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	var resp SendMessageResponse
	path := fmt.Sprintf("/dms/%s/messages", guildID)
	if err := c.apiPost(ctx, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AckInteraction acknowledges an interaction (button callback).
func (c *HTTPClient) AckInteraction(ctx context.Context, interactionID string) error {
	resp := InteractionResponse{Code: 0}
	body, _ := json.Marshal(resp)

	path := fmt.Sprintf("/interactions/%s", interactionID)
	return c.apiPut(ctx, path, body)
}

// UploadMedia uploads a file for later sending as a media message.
func (c *HTTPClient) UploadMedia(ctx context.Context, chatType string, openID string, mediaType int, fileURL string) (*UploadMediaResponse, error) {
	var path string
	switch chatType {
	case ChatTypeC2C:
		path = fmt.Sprintf("/v2/users/%s/files", openID)
	case ChatTypeGroup:
		path = fmt.Sprintf("/v2/groups/%s/files", openID)
	default:
		return nil, fmt.Errorf("unsupported chat type for upload: %s", chatType)
	}

	reqBody := map[string]interface{}{
		"file_type": mediaType,
		"url":       fileURL,
	}
	body, _ := json.Marshal(reqBody)

	var resp UploadMediaResponse
	if err := c.apiPost(ctx, path, body, &resp); err != nil {
		return nil, fmt.Errorf("upload media: %w", err)
	}
	return &resp, nil
}

// apiGet performs an authenticated GET request.
func (c *HTTPClient) apiGet(ctx context.Context, path string, result any) error {
	token, err := c.GetToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, APIBaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("api GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api GET %s: status %d: %s", path, resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response from %s: %w", path, err)
		}
	}
	return nil
}

// apiPost performs an authenticated POST request.
func (c *HTTPClient) apiPost(ctx context.Context, path string, body []byte, result any) error {
	token, err := c.GetToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("api POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		// Token might be expired; invalidate and let retry logic handle it
		if resp.StatusCode == http.StatusUnauthorized {
			c.InvalidateToken()
		}
		return fmt.Errorf("api POST %s: status %d: %s", path, resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response from %s: %w", path, err)
		}
	}
	return nil
}

// apiPut performs an authenticated PUT request.
func (c *HTTPClient) apiPut(ctx context.Context, path string, body []byte) error {
	token, err := c.GetToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, APIBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("api PUT %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api PUT %s: status %d: %s", path, resp.StatusCode, string(respBody))
	}
	return nil
}
