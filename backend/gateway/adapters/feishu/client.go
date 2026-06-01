package feishu

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

// Client handles Feishu REST API calls with token management.
type Client struct {
	httpClient  *http.Client
	appID       string
	appSecret   string
	baseURL     string

	mu            sync.Mutex
	tenantToken   string
	tenantExpires time.Time
}

// NewClient creates a Feishu API client.
func NewClient(appID, appSecret, baseURL string) *Client {
	if baseURL == "" {
		baseURL = APIBaseURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		appID:      appID,
		appSecret:  appSecret,
		baseURL:    baseURL,
	}
}

// GetToken obtains or refreshes the tenant access token.
func (c *Client) GetToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tenantToken != "" && time.Now().Before(c.tenantExpires) {
		return c.tenantToken, nil
	}

	reqBody := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+tokenAppAccessPath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("token error: code=%d msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	c.tenantToken = tokenResp.TenantAccessToken
	c.tenantExpires = time.Now().Add(time.Duration(tokenResp.Expire-60) * time.Second)

	return c.tenantToken, nil
}

// GetBotInfo fetches the bot's own information for validating credentials.
func (c *Client) GetBotInfo(ctx context.Context) (*BotInfo, error) {
	var resp struct {
		Code int     `json:"code"`
		Msg  string  `json:"msg"`
		Data BotInfo `json:"data"`
	}
	if err := c.apiGet(ctx, botInfoPath, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bot info: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return &resp.Data, nil
}

// GetWSGateway obtains the WebSocket gateway URL.
func (c *Client) GetWSGateway(ctx context.Context) (string, error) {
	var resp WSGatewayResponse
	if err := c.apiGet(ctx, wsGatewayPath, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("ws gateway: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data.URL, nil
}

// SendMessage sends a message via the IM API.
func (c *Client) SendMessage(ctx context.Context, receiveID, msgType, content string) (*SendMessageResponseData, error) {
	body, _ := json.Marshal(SendMessageRequest{
		ReceiveID: receiveID,
		MsgType:   msgType,
		Content:   content,
	})

	token, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+sendMessagePath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	var result SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode send response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("send message: code=%d msg=%s", result.Code, result.Msg)
	}

	return &result.Data, nil
}

func (c *Client) apiGet(ctx context.Context, path string, result any) error {
	token, err := c.GetToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("api GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
