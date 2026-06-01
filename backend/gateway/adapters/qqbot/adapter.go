package qqbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/insmtx/Leros/backend/gateway/pkg/core"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// Adapter implements the QQ Bot channel adapter.
//
// It satisfies:
//   - core.Connector  (identity metadata)
//   - core.Lifecycle  (WebSocket connect/disconnect/reconnect)
//   - core.Receiver   (WebSocket events → MessageEnvelope → callback)
//   - core.Sender     (REST API → QQ servers)
type Adapter struct {
	cfg    types.ChannelConfig
	client *HTTPClient
	logger *log.Logger

	// Gateway callback
	callback core.MessageCallback

	// WebSocket
	ws       *wsConn
	wsCtx    context.Context
	wsCancel context.CancelFunc

	// Reconnect
	reconnectMu      sync.Mutex
	reconnectDone    chan struct{}
	reconnectBackoff int
	reconnectCount   int

	// Runtime stats
	statsMu       sync.RWMutex
	connectTime   time.Time
	lastError     string
	quickDisconnects int  // consecutive disconnects within threshold
	lastDisconnect   time.Time
}

// NewAdapter creates a QQ Bot adapter from channel config.
func NewAdapter(cfg types.ChannelConfig) *Adapter {
	appID, _ := cfg.Extra["app_id"].(string)
	appSecret, _ := cfg.Extra["client_secret"].(string)

	return &Adapter{
		cfg: cfg,
		client: NewHTTPClient(appID, appSecret),
	}
}

// Info returns adapter metadata.
func (a *Adapter) Info() types.AdapterInfo {
	return types.AdapterInfo{
		Code:        "qqbot",
		Label:       "QQ Bot",
		Description: "QQ Bot channel via WebSocket gateway (C2C, group, guild, DM)",
		Version:     "1.0.0",
		Capabilities: types.ChannelCapabilities{
			SupportsIM:     true,
			SupportsWebhook: false,
			SupportsStream:  false,
			NeedsLongConn:   true,
			MaxMessageLen:   MaxMessageLength,
		},
	}
}

// OnMessage registers the inbound message callback.
func (a *Adapter) OnMessage(callback core.MessageCallback) error {
	if a.callback != nil {
		return fmt.Errorf("qqbot adapter: OnMessage already registered")
	}
	a.callback = callback
	return nil
}

// Connect starts the WebSocket connection and begins event processing.
func (a *Adapter) Connect(ctx context.Context) error {
	if a.callback == nil {
		return fmt.Errorf("qqbot adapter: OnMessage must be called before Connect")
	}

	a.wsCtx, a.wsCancel = context.WithCancel(ctx)
	a.reconnectDone = make(chan struct{})

	a.statsMu.Lock()
	a.connectTime = time.Now()
	a.reconnectCount = 0
	a.lastError = ""
	a.statsMu.Unlock()

	// Start the connect/reconnect loop
	go a.connectLoop()

	return nil
}

// Disconnect gracefully shuts down the WebSocket and reconnect loop.
func (a *Adapter) Disconnect(ctx context.Context) error {
	if a.wsCancel != nil {
		a.wsCancel()
	}
	if a.ws != nil {
		a.ws.close()
	}
	if a.reconnectDone != nil {
		<-a.reconnectDone
	}
	return nil
}

// Health checks whether the adapter is connected.
func (a *Adapter) Health(ctx context.Context) error {
	a.statsMu.RLock()
	defer a.statsMu.RUnlock()

	if a.ws == nil {
		return fmt.Errorf("qqbot: not connected")
	}
	if a.lastError != "" {
		return fmt.Errorf("qqbot: %s", a.lastError)
	}
	return nil
}

// Status returns detailed runtime status.
func (a *Adapter) Status() core.StatusDetail {
	a.statsMu.RLock()
	defer a.statsMu.RUnlock()

	var connected bool
	var sessionID string
	if a.ws != nil {
		a.ws.mu.Lock()
		connected = !a.ws.closed
		sessionID = a.ws.sessionID
		a.ws.mu.Unlock()
	}

	var uptime time.Duration
	if !a.connectTime.IsZero() {
		uptime = time.Since(a.connectTime)
	}

	return core.StatusDetail{
		Connected:      connected,
		SessionID:      sessionID,
		Uptime:         uptime,
		ReconnectCount: a.reconnectCount,
		LastError:      a.lastError,
	}
}

// connectLoop manages the connection lifecycle with reconnection logic.
func (a *Adapter) connectLoop() {
	defer close(a.reconnectDone)

	const quickDisconnectThreshold = 5 * time.Second
	const maxQuickDisconnects = 3

	for {
		select {
		case <-a.wsCtx.Done():
			return
		default:
		}

		a.logf("connecting to QQ Bot gateway (attempt %d)...", a.reconnectBackoff+1)

		ws := &wsConn{
			adapter:     a,
			connectAt:   time.Now(),
			reconnectCh: make(chan struct{}, 1),
		}
		a.ws = ws

		if err := ws.connect(a.wsCtx); err != nil {
			a.logf("connection failed: %v", err)
			a.setError(err.Error())
			a.handleReconnect()
			continue
		}

		// Connected successfully — reset quick disconnect counter
		a.statsMu.Lock()
		a.quickDisconnects = 0
		a.reconnectCount++
		a.statsMu.Unlock()

		// Wait for reconnect signal or context cancellation
		select {
		case <-ws.reconnectCh:
			duration := time.Since(ws.connectAt)
			a.logf("disconnected after %v", duration)

			// Quick disconnect detection
			a.statsMu.Lock()
			a.lastDisconnect = time.Now()
			if duration < quickDisconnectThreshold {
				a.quickDisconnects++
			} else {
				a.quickDisconnects = 0
			}
			qdCount := a.quickDisconnects
			a.statsMu.Unlock()

			if qdCount >= maxQuickDisconnects {
				a.logf("too many quick disconnects (%d), pausing...", qdCount)
				a.setError("too many quick disconnects")
				a.handleReconnect()
			} else {
				a.handleReconnect()
			}

			ws.close()
		case <-a.wsCtx.Done():
			ws.close()
			return
		}
	}
}

// handleReconnect applies backoff and resets state.
func (a *Adapter) handleReconnect() {
	a.reconnectMu.Lock()
	defer a.reconnectMu.Unlock()

	a.reconnectBackoff++
	if a.reconnectBackoff >= len(reconnectBackoff) {
		a.reconnectBackoff = len(reconnectBackoff) - 1
	}

	// Extra delay if quick disconnects are happening
	delay := reconnectBackoff[a.reconnectBackoff]
	a.statsMu.RLock()
	if a.quickDisconnects >= 3 {
		delay = 60
	}
	a.statsMu.RUnlock()

	a.logf("reconnecting in %d seconds...", delay)
	a.setError("")

	select {
	case <-a.wsCtx.Done():
		return
	case <-time.After(time.Duration(delay) * time.Second):
	}
}

// --- Event Handlers (called by wsConn) ---

func (a *Adapter) handleC2CMessage(ctx context.Context, payload *WSPayload) {
	var evt C2CMessageEvent
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		a.logf("unmarshal c2c message: %v", err)
		return
	}

	raw, _ := json.MarshalIndent(evt, "", "  ")
	a.logf("[IN] C2C_MESSAGE:\n%s", string(raw))

	env := a.normalizeC2C(&evt)
	if env == nil {
		return
	}

	normalized, _ := json.MarshalIndent(env, "", "  ")
	a.logf("[OUT] MessageEnvelope:\n%s", string(normalized))

	a.dispatch(ctx, env)
}

func (a *Adapter) handleGroupMessage(ctx context.Context, payload *WSPayload) {
	var evt GroupAtMessageEvent
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		a.logf("unmarshal group message: %v", err)
		return
	}

	raw, _ := json.MarshalIndent(evt, "", "  ")
	a.logf("[IN] GROUP_AT_MESSAGE:\n%s", string(raw))

	env := a.normalizeGroup(&evt)
	if env == nil {
		return
	}

	normalized, _ := json.MarshalIndent(env, "", "  ")
	a.logf("[OUT] MessageEnvelope:\n%s", string(normalized))

	a.dispatch(ctx, env)
}

func (a *Adapter) handleGuildMessage(ctx context.Context, payload *WSPayload) {
	var evt GuildAtMessageEvent
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		a.logf("unmarshal guild message: %v", err)
		return
	}

	raw, _ := json.MarshalIndent(evt, "", "  ")
	a.logf("[IN] GUILD_AT_MESSAGE:\n%s", string(raw))

	env := a.normalizeGuild(&evt)
	if env == nil {
		return
	}

	normalized, _ := json.MarshalIndent(env, "", "  ")
	a.logf("[OUT] MessageEnvelope:\n%s", string(normalized))

	a.dispatch(ctx, env)
}

func (a *Adapter) handleDMMessage(ctx context.Context, payload *WSPayload) {
	var evt DirectMessageEvent
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		a.logf("unmarshal dm message: %v", err)
		return
	}

	raw, _ := json.MarshalIndent(evt, "", "  ")
	a.logf("[IN] DIRECT_MESSAGE:\n%s", string(raw))

	env := a.normalizeDM(&evt)
	if env == nil {
		return
	}

	normalized, _ := json.MarshalIndent(env, "", "  ")
	a.logf("[OUT] MessageEnvelope:\n%s", string(normalized))

	a.dispatch(ctx, env)
}

func (a *Adapter) handleInteraction(ctx context.Context, payload *WSPayload) {
	var evt InteractionEvent
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		a.logf("unmarshal interaction: %v", err)
		return
	}

	raw, _ := json.MarshalIndent(evt, "", "  ")
	a.logf("[IN] INTERACTION:\n%s", string(raw))

	// ACK the interaction immediately
	go func() {
		if err := a.client.AckInteraction(context.Background(), evt.ID); err != nil {
			a.logf("ack interaction %s: %v", evt.ID, err)
		}
	}()

	env := a.normalizeInteraction(&evt)
	if env == nil {
		return
	}

	normalized, _ := json.MarshalIndent(env, "", "  ")
	a.logf("[OUT] MessageEnvelope:\n%s", string(normalized))

	a.dispatch(ctx, env)
}

// dispatch sends a normalized message to the gateway callback.
func (a *Adapter) dispatch(ctx context.Context, env *types.MessageEnvelope) {
	if a.callback == nil {
		return
	}
	if err := a.callback(ctx, env); err != nil {
		a.logf("dispatch error for msg %s: %v", env.MessageID, err)
	}
}

// extractText strips @bot mentions from message content.
func (a *Adapter) extractText(content string) string {
	// Remove <@!bot_id> mentions and @bot_name prefixes
	text := strings.TrimSpace(content)
	// QQ sends mentions as <@!id> in content; strip them
	for strings.Contains(text, "<@!") {
		idx := strings.Index(text, "<@!")
		end := strings.Index(text[idx:], ">")
		if end == -1 {
			break
		}
		text = text[:idx] + text[idx+end+1:]
	}

	// Also handle plain @mentions at start
	if idx := strings.Index(text, "@"); idx >= 0 {
		// Find word boundary after @
		rest := text[idx+1:]
		spaceIdx := strings.IndexAny(rest, " \t\n")
		if spaceIdx > 0 {
			text = text[:idx] + rest[spaceIdx:]
		} else if spaceIdx == -1 {
			text = text[:idx] // whole remainder is the mention
		}
	}

	return strings.TrimSpace(text)
}

// genID generates a unique message/trace ID.
func genID() string {
	return uuid.New().String()
}

// setError updates the last error in the runtime stats.
func (a *Adapter) setError(msg string) {
	a.statsMu.Lock()
	defer a.statsMu.Unlock()
	a.lastError = msg
}

// logf writes a formatted log message.
func (a *Adapter) logf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(format, args...)
	} else {
		log.Printf("[qqbot] "+format, args...)
	}
}
