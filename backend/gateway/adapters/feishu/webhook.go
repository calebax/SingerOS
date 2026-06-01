package feishu

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
	"github.com/insmtx/Leros/backend/gateway/pkg/webhook"
)

// handleWebhook processes incoming Feishu webhook HTTP requests.
//
// Security chain: body size → rate limit → content type → dedup →
// verification token → URL verification (challenge) → signature.
func (a *Adapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
	key := fmt.Sprintf("feishu:%s:%s", r.URL.Path, r.RemoteAddr)

	// 1. Guard checks (body size, content type, rate limit, dedup)
	body, err := a.guard.Guard(r, key)
	if err != nil {
		if gerr, ok := err.(*webhook.GuardError); ok {
			http.Error(w, gerr.Error(), http.StatusTooManyRequests)
		} else {
			http.Error(w, "guard error", http.StatusBadRequest)
		}
		return
	}

	// 2. Parse event
	var payload struct {
		Schema string          `json:"schema"`
		Header EventHeader     `json:"header"`
		Event  json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// 3. Verification token check
	verificationToken, _ := a.cfg.Extra["verification_token"].(string)
	if verificationToken != "" {
		if payload.Header.Token != verificationToken {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
	}

	// 4. Signature verification
	if err := a.verifySignature(r, body); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// 5. URL verification (challenge response)
	if payload.Header.EventType == EventURLVerification {
		var challenge struct {
			Challenge string `json:"challenge"`
			Token     string `json:"token"`
			Type      string `json:"type"`
		}
		if err := json.Unmarshal(payload.Event, &challenge); err == nil {
			resp := map[string]string{"challenge": challenge.Challenge}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	// 6. Normalize and dispatch
	env := a.normalizeWebhookEvent(&payload)
	if env == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0}`))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	a.dispatch(ctx, env)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"code":0}`))
}

// normalizeWebhookEvent converts a webhook event to a MessageEnvelope.
func (a *Adapter) normalizeWebhookEvent(payload *struct {
	Schema string          `json:"schema"`
	Header EventHeader     `json:"header"`
	Event  json.RawMessage `json:"event"`
}) *types.MessageEnvelope {
	switch payload.Header.EventType {
	case EventMessageReceived:
		var msg MessageEvent
		if err := json.Unmarshal(payload.Event, &msg); err != nil {
			return nil
		}
		if msg.Sender.SenderType == "app" {
			return nil // ignore self-messages
		}
		return a.normalizeMessage(&msg)

	default:
		// Non-message events: publish as generic event
		return a.normalizeGenericEvent(&payload.Header, payload.Event)
	}
}

// normalizeGenericEvent wraps a non-message event into a MessageEnvelope.
func (a *Adapter) normalizeGenericEvent(header *EventHeader, event json.RawMessage) *types.MessageEnvelope {
	return &types.MessageEnvelope{
		MessageID:   header.EventID,
		TraceID:     uuid.New().String(),
		Channel:     "feishu",
		MessageType: types.MessageTypeEvent,
		SessionKey: types.SessionKey{
			Channel: "feishu",
		},
		Content: types.MessageContent{
			Extra: map[string]any{
				"feishu_event_type": header.EventType,
			},
		},
		CommandHint: stringPtr(header.EventType),
		RawEvent:    event,
		ReceivedAt:  time.Now(),
	}
}

// verifySignature checks the Feishu signature header.
// Algorithm: SHA256(timestamp + nonce + encrypt_key + body)
func (a *Adapter) verifySignature(r *http.Request, body []byte) error {
	timestamp := r.Header.Get("X-Lark-Request-Timestamp")
	nonce := r.Header.Get("X-Lark-Request-Nonce")
	signature := r.Header.Get("X-Lark-Signature")

	if timestamp == "" || nonce == "" || signature == "" {
		return nil // no signature provided, skip
	}

	// Timestamp freshness check (5 min)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	if time.Now().Unix()-ts > 300 || ts-time.Now().Unix() > 300 {
		return fmt.Errorf("timestamp out of range")
	}

	encryptKey, _ := a.cfg.Extra["encrypt_key"].(string)
	if encryptKey == "" {
		encryptKey = "" // empty string is valid when no encryption
	}

	sigBase := timestamp + nonce + encryptKey + string(body)
	hash := sha256.Sum256([]byte(sigBase))
	expected := fmt.Sprintf("%x", hash)

	if signature != expected {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
