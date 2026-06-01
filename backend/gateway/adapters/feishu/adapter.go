package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/insmtx/Leros/backend/gateway/pkg/core"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
	"github.com/insmtx/Leros/backend/gateway/pkg/webhook"
)

// Adapter implements the Feishu/Lark channel adapter.
//
// It supports both WebSocket long connection and Webhook HTTP modes.
// Implements: Connector, Lifecycle, Receiver, Sender, WebhookReceiver.
type Adapter struct {
	cfg    types.ChannelConfig
	client *Client
	guard  *webhook.Guard
	logger *log.Logger

	callback core.MessageCallback

	// WebSocket
	wsDone   chan struct{}
	wsCancel context.CancelFunc

	// Runtime
	statsMu       sync.RWMutex
	connectTime   time.Time
	lastError     string
	reconnectCount int
	sessionID     string
}

// NewAdapter creates a Feishu adapter from channel config.
func NewAdapter(cfg types.ChannelConfig) *Adapter {
	appID, _ := cfg.Extra["app_id"].(string)
	appSecret, _ := cfg.Extra["app_secret"].(string)
	baseURL, _ := cfg.Extra["domain"].(string)

	guard := webhook.NewGuard(webhook.GuardConfig{
		MaxBodySize: 1 << 20, // 1MB
		RateLimit:   120,
		RateWindow:  60 * time.Second,
		DedupTTL:    10 * time.Minute,
	})

	return &Adapter{
		cfg:    cfg,
		client: NewClient(appID, appSecret, baseURL),
		guard:  guard,
	}
}

// Info returns adapter metadata.
func (a *Adapter) Info() types.AdapterInfo {
	return types.AdapterInfo{
		Code:        "feishu",
		Label:       "Feishu/Lark",
		Description: "Feishu/Lark IM bot via WebSocket + webhook dual mode",
		Version:     "1.0.0",
		Capabilities: types.ChannelCapabilities{
			SupportsIM:     true,
			SupportsWebhook: true,
			SupportsStream:  false,
			NeedsLongConn:   true,
			MaxMessageLen:   20000,
		},
	}
}

// OnMessage registers the inbound message callback.
func (a *Adapter) OnMessage(callback core.MessageCallback) error {
	if a.callback != nil {
		return fmt.Errorf("feishu adapter: OnMessage already registered")
	}
	a.callback = callback
	return nil
}

// Connect verifies credentials and starts the connection (WS or webhook).
func (a *Adapter) Connect(ctx context.Context) error {
	if a.callback == nil {
		return fmt.Errorf("feishu adapter: OnMessage must be called before Connect")
	}

	// Verify credentials
	if _, err := a.client.GetBotInfo(ctx); err != nil {
		return fmt.Errorf("feishu: credential validation failed: %w", err)
	}

	a.statsMu.Lock()
	a.connectTime = time.Now()
	a.lastError = ""
	a.statsMu.Unlock()

	mode, _ := a.cfg.Extra["connection_mode"].(string)
	if mode == "webhook" {
		// Webhook mode: just register the route; server handles it
		return nil
	}

	// Default: WebSocket mode
	a.wsDone = make(chan struct{})
	wsCtx, cancel := context.WithCancel(ctx)
	a.wsCancel = cancel

	go a.wsLoop(wsCtx)
	return nil
}

// Disconnect shuts down the connection.
func (a *Adapter) Disconnect(ctx context.Context) error {
	if a.wsCancel != nil {
		a.wsCancel()
	}
	if a.wsDone != nil {
		<-a.wsDone
	}
	return nil
}

// Health checks whether the adapter is functional.
func (a *Adapter) Health(ctx context.Context) error {
	a.statsMu.RLock()
	defer a.statsMu.RUnlock()

	if a.lastError != "" {
		return fmt.Errorf("feishu: %s", a.lastError)
	}
	return nil
}

// Status returns detailed runtime status.
func (a *Adapter) Status() core.StatusDetail {
	a.statsMu.RLock()
	defer a.statsMu.RUnlock()

	connected := a.lastError == ""
	var uptime time.Duration
	if !a.connectTime.IsZero() {
		uptime = time.Since(a.connectTime)
	}

	return core.StatusDetail{
		Connected:      connected,
		SessionID:      a.sessionID,
		Uptime:         uptime,
		ReconnectCount: a.reconnectCount,
		LastError:      a.lastError,
	}
}

// RegisterWebhookRoutes registers the webhook HTTP route.
func (a *Adapter) RegisterWebhookRoutes(mux *http.ServeMux) error {
	mux.HandleFunc("/webhooks/feishu", a.handleWebhook)
	return nil
}

// --- WebSocket Loop ---

func (a *Adapter) wsLoop(ctx context.Context) {
	defer close(a.wsDone)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wsURL, err := a.client.GetWSGateway(ctx)
		if err != nil {
			a.logf("get ws gateway: %v", err)
			a.setError(err.Error())
			time.Sleep(10 * time.Second)
			continue
		}

		a.logf("connecting to Feishu WebSocket: %s", wsURL[:50]+"...")
		if err := a.runWS(ctx, wsURL); err != nil {
			a.logf("ws error: %v", err)
			a.setError(err.Error())
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (a *Adapter) runWS(ctx context.Context, wsURL string) error {
	// TODO: integrate a gorilla/websocket-based Feishu WS client
	// The Feishu WS protocol sends JSON frames:
	//   {"type":"message","data":{"schema":"2.0","header":{...},"event":{...}}}
	// For now, place the connection loop structure:
	a.logf("Feishu WebSocket loop started")
	<-ctx.Done()
	return nil
}

// --- Event dispatch ---

func (a *Adapter) dispatch(ctx context.Context, env *types.MessageEnvelope) {
	if a.callback == nil {
		return
	}
	if err := a.callback(ctx, env); err != nil {
		a.logf("dispatch error: %v", err)
	}
}

// --- Normalization helpers ---

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func genID() string {
	return uuid.New().String()
}

func stringPtr(s string) *string {
	return &s
}

func (a *Adapter) setError(msg string) {
	a.statsMu.Lock()
	defer a.statsMu.Unlock()
	a.lastError = msg
}

func (a *Adapter) logf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(format, args...)
	} else {
		log.Printf("[feishu] "+format, args...)
	}
}
