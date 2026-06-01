package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BridgeClient communicates with the Node bridge via HTTP REST.
type BridgeClient struct {
	baseURL string
	client  *http.Client
}

// NewBridgeClient creates a bridge HTTP client.
func NewBridgeClient(port int) *BridgeClient {
	return &BridgeClient{
		baseURL: fmt.Sprintf("http://localhost:%d", port),
		client:  &http.Client{Timeout: bridgeAPITimeout},
	}
}

// Health checks the bridge health.
func (c *BridgeClient) Health(ctx context.Context) (*BridgeHealth, error) {
	var health BridgeHealth
	if err := c.get(ctx, bridgeHealthPath, &health); err != nil {
		return nil, err
	}
	return &health, nil
}

// GetMessages fetches pending messages from the bridge.
func (c *BridgeClient) GetMessages(ctx context.Context, cursor string) ([]BridgeMessage, string, bool, error) {
	path := bridgeMessagesPath
	if cursor != "" {
		path += "?cursor=" + cursor
	}

	var resp BridgeMessagesResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, cursor, false, err
	}
	return resp.Messages, resp.Cursor, resp.HasMore, nil
}

// SendText sends a text message.
func (c *BridgeClient) SendText(ctx context.Context, req BridgeSendRequest) (*BridgeSendResponse, error) {
	body, _ := json.Marshal(req)
	var resp BridgeSendResponse
	if err := c.post(ctx, bridgeSendPath, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendMedia sends a media message.
func (c *BridgeClient) SendMedia(ctx context.Context, req BridgeSendMediaRequest) (*BridgeSendResponse, error) {
	body, _ := json.Marshal(req)
	var resp BridgeSendResponse
	if err := c.post(ctx, bridgeSendMediaPath, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *BridgeClient) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("bridge GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bridge GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

func (c *BridgeClient) post(ctx context.Context, path string, body []byte, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("bridge POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bridge POST %s: status %d: %s", path, resp.StatusCode, string(respBody))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
