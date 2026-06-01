// Package github implements a GitHub webhook adapter for the message gateway.
//
// It receives GitHub webhook events, verifies HMAC signatures, normalizes
// the events into MessageEnvelope, and passes them to the gateway for dispatch.
//
// This adapter implements:
//   - core.Connector  (identity metadata)
//   - core.Receiver   (webhook → MessageEnvelope → callback)
//   - core.Lifecycle  (start/stop HTTP server, but no long-lived connection)
//
// It does NOT implement core.Sender — GitHub is an inbound-only channel.
package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/insmtx/Leros/backend/gateway/pkg/core"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// Adapter receives GitHub webhook events and converts them to MessageEnvelope.
type Adapter struct {
	cfg      types.ChannelConfig
	callback core.MessageCallback
	server   *http.Server
}

// Info returns adapter metadata.
func (a *Adapter) Info() types.AdapterInfo {
	return types.AdapterInfo{
		Code:        "github",
		Label:       "GitHub",
		Description: "Receives GitHub webhook events (PR, issue, push, etc.)",
		Version:     "1.0.0",
		Capabilities: types.ChannelCapabilities{
			SupportsWebhook: true,
			SupportsStream:  false,
			NeedsLongConn:   false,
			MaxMessageLen:   0, // no limit for event payloads
		},
	}
}

// OnMessage registers the inbound message callback.
func (a *Adapter) OnMessage(callback core.MessageCallback) error {
	if a.callback != nil {
		return fmt.Errorf("github adapter: OnMessage already registered")
	}
	a.callback = callback
	return nil
}

// Connect starts the HTTP webhook server.
//
// The GitHub adapter runs its own HTTP server (not embedded in the gateway
// process's Gin router) because adapters are self-contained. For production
// deployments, you may prefer a reverse proxy routing to adapter endpoints.
func (a *Adapter) Connect(ctx context.Context) error {
	if a.callback == nil {
		return fmt.Errorf("github adapter: OnMessage must be called before Connect")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.handleHealth)
	mux.HandleFunc("/webhook", a.handleWebhook)

	host := "0.0.0.0"
	port := "8081"
	if v, ok := a.cfg.Extra["bind_host"].(string); ok && v != "" {
		host = v
	}
	if v, ok := a.cfg.Extra["bind_port"].(string); ok && v != "" {
		port = v
	}

	a.server = &http.Server{
		Addr:    host + ":" + port,
		Handler: mux,
	}

	go func() {
		fmt.Printf("GitHub adapter listening on %s\n", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("GitHub adapter server error: %v\n", err)
		}
	}()

	return nil
}

// Disconnect shuts down the HTTP server gracefully.
func (a *Adapter) Disconnect(ctx context.Context) error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

// Health returns nil if the adapter is functional.
func (a *Adapter) Health(ctx context.Context) error {
	// For a webhook receiver, "healthy" means the HTTP server is running.
	if a.server == nil {
		return fmt.Errorf("github adapter: server not started")
	}
	return nil
}

// handleHealth responds to health check requests.
func (a *Adapter) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "channel": "github"})
}

// handleWebhook processes incoming GitHub webhook events.
//
// Flow:
//  1. Verify HMAC-SHA256 signature
//  2. Read and parse the raw payload
//  3. Determine the event type from headers
//  4. Build a normalized MessageEnvelope
//  5. Pass to the callback for dispatch
func (a *Adapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Verify signature
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := a.verifySignature(body, r.Header.Get("X-Hub-Signature-256")); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// 2. Determine event type
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	// 3. Build MessageEnvelope
	env := a.buildEnvelope(eventType, body, r.Header.Get("X-GitHub-Delivery"))

	// 4. Dispatch
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := a.callback(ctx, env); err != nil {
		fmt.Printf("GitHub adapter: dispatch error for %s: %v\n", env.MessageID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// verifySignature checks the HMAC-SHA256 signature of the webhook payload.
func (a *Adapter) verifySignature(body []byte, signatureHeader string) error {
	secret, _ := a.cfg.Extra["webhook_secret"].(string)
	if secret == "" {
		// Skip verification if no secret is configured (development only)
		return nil
	}

	if signatureHeader == "" {
		return fmt.Errorf("missing signature header")
	}

	const prefix = "sha256="
	if len(signatureHeader) < len(prefix) || signatureHeader[:len(prefix)] != prefix {
		return fmt.Errorf("invalid signature format")
	}

	expectedMAC, err := hex.DecodeString(signatureHeader[len(prefix):])
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actualMAC := mac.Sum(nil)

	if !hmac.Equal(actualMAC, expectedMAC) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// buildEnvelope normalizes a raw GitHub webhook event into a MessageEnvelope.
func (a *Adapter) buildEnvelope(eventType string, body []byte, deliveryID string) *types.MessageEnvelope {
	msgID := deliveryID
	if msgID == "" {
		msgID = uuid.New().String()
	}

	// Extract actor and repository from the payload
	actor, repo := extractActorAndRepo(body)

	env := &types.MessageEnvelope{
		MessageID:   msgID,
		TraceID:     uuid.New().String(),
		Channel:     "github",
		MessageType: toMessageType(eventType),
		SessionKey: types.SessionKey{
			Channel:  "github",
			ChatType: types.ChatTypeChannel,
			ChatID:   repo,
			UserID:   actor,
		},
		Sender: types.SenderInfo{
			UserID:   actor,
			Username: actor,
		},
		Content: types.MessageContent{
			Text: fmt.Sprintf("[%s] event on %s", eventType, repo),
			Extra: map[string]any{
				"github_event": eventType,
			},
		},
		RawEvent:   body,
		ReceivedAt: time.Now(),
	}

	return env
}

// toMessageType maps GitHub event types to normalized message types.
func toMessageType(eventType string) types.MessageType {
	switch eventType {
	case "issues", "issue_comment":
		return types.MessageTypeEvent
	case "pull_request", "pull_request_review", "pull_request_review_comment":
		return types.MessageTypeEvent
	case "push":
		return types.MessageTypeEvent
	default:
		return types.MessageTypeEvent
	}
}

// extractActorAndRepo extracts the actor login and repository full name from
// a JSON webhook payload.
func extractActorAndRepo(body []byte) (actor, repo string) {
	var payload struct {
		Sender struct {
			Login string `json:"login"`
		} `json:"sender"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		actor = payload.Sender.Login
		repo = payload.Repository.FullName
	}
	if actor == "" {
		actor = "unknown"
	}
	if repo == "" {
		repo = "unknown"
	}
	return
}

// NewAdapter creates a GitHub adapter from configuration.
func NewAdapter(cfg types.ChannelConfig) *Adapter {
	return &Adapter{
		cfg: cfg,
	}
}
