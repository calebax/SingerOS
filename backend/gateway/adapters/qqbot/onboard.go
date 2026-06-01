package qqbot

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/insmtx/Leros/backend/gateway/pkg/onboard"
)

// QQ Bot API endpoints used during onboarding (on the portal host, not the API host).
const (
	onboardPortal    = "https://q.qq.com"
	createBindTaskPath = "/lite/create_bind_task"
	pollBindResultPath = "/lite/poll_bind_result"
)

// Onboarder implements onboard.PlatformOnboarder for QQ Bot.
//
// The QQ Bot scan-to-configure flow is:
//  1. create_bind_task → get task_id + AES key
//  2. Display QR code (user scans in QQ app)
//  3. poll_bind_result → get encrypted client_secret
//  4. AES-256-GCM decrypt → get plaintext client_secret
type Onboarder struct {
	client *http.Client
}

// NewOnboarder creates a QQ Bot onboard handler.
func NewOnboarder() *Onboarder {
	return &Onboarder{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// PlatformCode returns "qqbot".
func (o *Onboarder) PlatformCode() string {
	return "qqbot"
}

// onboardState holds the Init result for the QQ Bot flow.
type onboardState struct {
	taskID string
	aesKey []byte // 32-byte AES-256 key
}

// Init creates a bind task on QQ's servers and generates an AES key for decryption.
func (o *Onboarder) Init(ctx context.Context) (any, error) {
	// Generate random 256-bit AES key
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("generate aes key: %w", err)
	}
	keyBase64 := base64.StdEncoding.EncodeToString(aesKey)

	// POST create_bind_task
	reqBody := map[string]string{"key": keyBase64}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, onboardPortal+createBindTaskPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create_bind_task: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("create_bind_task: status %d: %s", resp.StatusCode, string(respBody))
	}

	var data struct {
		Retcode int `json:"retcode"`
		Msg     string `json:"msg"`
		Data    struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("parse create_bind_task response: %w", err)
	}
	if data.Retcode != 0 {
		return nil, fmt.Errorf("create_bind_task failed: %s", data.Msg)
	}

	return &onboardState{
		taskID: data.Data.TaskID,
		aesKey: aesKey,
	}, nil
}

// Begin returns the QR code URL for the user to scan.
func (o *Onboarder) Begin(ctx context.Context, state any) (qrURL, manualURL string, pollToken any, err error) {
	st, ok := state.(*onboardState)
	if !ok {
		return "", "", nil, fmt.Errorf("invalid state type")
	}

	qrURL = fmt.Sprintf(
		"https://q.qq.com/qqbot/openclaw/connect.html?task_id=%s&_wv=2&source=singeros",
		st.taskID,
	)

	return qrURL, qrURL, st.taskID, nil
}

// Poll checks the bind result.
func (o *Onboarder) Poll(ctx context.Context, state any, pollToken any) (onboard.Status, *onboard.Result, error) {
	if _, ok := state.(*onboardState); !ok {
		return onboard.StatusFailed, nil, fmt.Errorf("invalid state type")
	}
	taskID, ok := pollToken.(string)
	if !ok {
		return onboard.StatusFailed, nil, fmt.Errorf("invalid poll token type")
	}

	reqBody := map[string]string{"task_id": taskID}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, onboardPortal+pollBindResultPath, bytes.NewReader(body))
	if err != nil {
		return onboard.StatusFailed, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return onboard.StatusFailed, nil, fmt.Errorf("poll_bind_result: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return onboard.StatusFailed, nil, fmt.Errorf("poll_bind_result: status %d: %s", resp.StatusCode, string(respBody))
	}

	var data struct {
		Retcode int `json:"retcode"`
		Msg     string `json:"msg"`
		Data    struct {
			Status            int    `json:"status"`
			BotAppID          string `json:"bot_appid"`
			BotEncryptSecret  string `json:"bot_encrypt_secret"`
			UserOpenID        string `json:"user_openid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return onboard.StatusFailed, nil, fmt.Errorf("parse poll_bind_result: %w", err)
	}
	if data.Retcode != 0 {
		return onboard.StatusFailed, nil, fmt.Errorf("poll_bind_result failed: %s", data.Msg)
	}

	switch data.Data.Status {
	case 0: // NONE
		return onboard.StatusPending, nil, nil
	case 1: // PENDING
		return onboard.StatusPending, nil, nil
	case 2: // COMPLETED
		return onboard.StatusCompleted, &onboard.Result{
			Platform: "qqbot",
			Credentials: map[string]string{
				"app_id":              data.Data.BotAppID,
				"encrypted_secret":    data.Data.BotEncryptSecret,
			},
			UserOpenID: data.Data.UserOpenID,
		}, nil
	case 3: // EXPIRED
		return onboard.StatusExpired, nil, nil
	default:
		return onboard.StatusFailed, nil, fmt.Errorf("unknown bind status: %d", data.Data.Status)
	}
}

// Decrypt decrypts the encrypted client_secret using AES-256-GCM.
func (o *Onboarder) Decrypt(ctx context.Context, state any, raw *onboard.Result) (*onboard.Result, error) {
	st, ok := state.(*onboardState)
	if !ok {
		return raw, nil // no state, no decryption needed
	}

	encryptedSecret, ok := raw.Credentials["encrypted_secret"]
	if !ok {
		return raw, nil // not encrypted
	}

	decrypted, err := aesGCMDecrypt(encryptedSecret, st.aesKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt client_secret: %w", err)
	}

	delete(raw.Credentials, "encrypted_secret")
	raw.Credentials["client_secret"] = decrypted
	raw.Credentials["app_id"] = raw.Credentials["app_id"] // already present

	return raw, nil
}

// Probe verifies the credentials by attempting to get an access token.
func (o *Onboarder) Probe(ctx context.Context, result *onboard.Result) error {
	appID := result.Credentials["app_id"]
	clientSecret := result.Credentials["client_secret"]
	if appID == "" || clientSecret == "" {
		return fmt.Errorf("missing credentials for probe")
	}

	// Try getting an access token to verify credentials
	client := NewHTTPClient(appID, clientSecret)
	_, err := client.GetToken(ctx)
	return err
}

// aesGCMDecrypt decrypts a base64-encoded AES-256-GCM ciphertext.
// Ciphertext layout: IV (12 bytes) ‖ ciphertext (N bytes) ‖ AuthTag (16 bytes)
func aesGCMDecrypt(encryptedBase64 string, key []byte) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	if len(raw) < 12+16 {
		return "", fmt.Errorf("ciphertext too short (%d bytes)", len(raw))
	}

	iv := raw[:12]
	ciphertextWithTag := raw[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, iv, ciphertextWithTag, nil)
	if err != nil {
		return "", fmt.Errorf("GCM decrypt: %w", err)
	}

	return string(plaintext), nil
}
