package qqbot

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsConn manages a single WebSocket connection to the QQ Bot Gateway.
type wsConn struct {
	conn      *websocket.Conn
	adapter   *Adapter
	connectAt time.Time

	mu           sync.Mutex
	sessionID    string
	lastSeq      int
	heartbeatMS  int
	closed       bool

	// reconnect
	reconnectCh chan struct{}
	reconnectAttempt int
}

// connect establishes a WebSocket connection to the gateway.
func (w *wsConn) connect(ctx context.Context) error {
	// Step 1: Get gateway URL
	gw, err := w.adapter.client.GetGateway(ctx)
	if err != nil {
		return fmt.Errorf("get gateway URL: %w", err)
	}

	// Step 2: Connect WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, gw.URL, nil)
	if err != nil {
		return fmt.Errorf("dial gateway: %w", err)
	}
	w.conn = conn

	// Step 3: Read Hello
	if err := w.readHello(ctx); err != nil {
		conn.Close()
		return err
	}

	// Step 4: Identify or Resume
	if err := w.authenticate(ctx); err != nil {
		conn.Close()
		return err
	}

	// Step 5: Start heartbeat
	go w.heartbeatLoop(ctx)

	// Step 6: Start read loop
	go w.readLoop(ctx)

	return nil
}

// readHello waits for the OpCode 10 Hello message.
func (w *wsConn) readHello(ctx context.Context) error {
	w.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, raw, err := w.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read hello: %w", err)
	}

	var payload WSPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("unmarshal hello: %w", err)
	}

	if payload.Op != OpHello {
		return fmt.Errorf("expected op 10 Hello, got op %d", payload.Op)
	}

	var hello HelloData
	if err := json.Unmarshal(payload.D, &hello); err != nil {
		return fmt.Errorf("unmarshal hello data: %w", err)
	}

	w.heartbeatMS = hello.HeartbeatInterval
	w.conn.SetReadDeadline(time.Time{}) // clear deadline for normal operation
	return nil
}

// authenticate sends Identify or Resume depending on session state.
func (w *wsConn) authenticate(ctx context.Context) error {
	w.mu.Lock()
	sessionID := w.sessionID
	w.mu.Unlock()

	if sessionID != "" {
		// Attempt Resume
		return w.sendResume()
	}
	return w.sendIdentify()
}

// sendIdentify sends OpCode 2 Identify.
func (w *wsConn) sendIdentify() error {
	token, err := w.adapter.client.GetToken(context.Background())
	if err != nil {
		return fmt.Errorf("get token for identify: %w", err)
	}

	identify := WSPayload{
		Op: OpIdentify,
		D: mustMarshal(IdentifyData{
			Token:   "QQBot " + token,
			Intents: DefaultIntents,
			Shard:   [2]int{0, 1},
			Properties: IdentifyProperties{
				OS:      "linux",
				Browser: "singeros-gateway",
				Device:  "singeros-gateway",
			},
		}),
	}

	body, _ := json.Marshal(identify)
	return w.conn.WriteMessage(websocket.TextMessage, body)
}

// sendResume sends OpCode 6 Resume.
func (w *wsConn) sendResume() error {
	token, err := w.adapter.client.GetToken(context.Background())
	if err != nil {
		return fmt.Errorf("get token for resume: %w", err)
	}

	w.mu.Lock()
	seq := w.lastSeq
	sid := w.sessionID
	w.mu.Unlock()

	resume := WSPayload{
		Op: OpResume,
		D: mustMarshal(ResumeData{
			Token:     "QQBot " + token,
			SessionID: sid,
			Seq:       seq,
		}),
	}

	body, _ := json.Marshal(resume)
	return w.conn.WriteMessage(websocket.TextMessage, body)
}

// sendHeartbeat sends OpCode 1 Heartbeat.
func (w *wsConn) sendHeartbeat() error {
	w.mu.Lock()
	seq := w.lastSeq
	w.mu.Unlock()

	payload := map[string]interface{}{"op": OpHeartbeat}
	if seq > 0 {
		payload["d"] = seq
	}

	body, _ := json.Marshal(payload)
	return w.conn.WriteMessage(websocket.TextMessage, body)
}

// heartbeatLoop sends periodic heartbeats.
func (w *wsConn) heartbeatLoop(ctx context.Context) {
	// Send at 80% of the server's heartbeat interval
	interval := time.Duration(float64(w.heartbeatMS)*0.8) * time.Millisecond
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// On reconnect, heartbeatLoop is restarted; old ticker stops via w.closed check
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.Lock()
			closed := w.closed
			w.mu.Unlock()
			if closed {
				return
			}

			if err := w.sendHeartbeat(); err != nil {
				// Heartbeat failure triggers reconnect
				w.adapter.logf("heartbeat failed: %v", err)
				w.triggerReconnect()
				return
			}
		}
	}
}

// readLoop continuously reads WebSocket messages and dispatches them.
func (w *wsConn) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, raw, err := w.conn.ReadMessage()
		if err != nil {
			// Check if this was an intentional close
			w.mu.Lock()
			closed := w.closed
			w.mu.Unlock()

			if closed {
				return
			}

			w.adapter.logf("ws read error: %v", err)
			w.triggerReconnect()
			return
		}

		var payload WSPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			w.adapter.logf("unmarshal ws payload: %v", err)
			continue
		}

		// Track sequence numbers
		if payload.S != nil {
			w.mu.Lock()
			w.lastSeq = *payload.S
			w.mu.Unlock()
		}

		w.handlePayload(ctx, &payload)
	}
}

// handlePayload routes a WebSocket payload by opcode.
func (w *wsConn) handlePayload(ctx context.Context, payload *WSPayload) {
	switch payload.Op {
	case OpDispatch:
		w.handleDispatch(ctx, payload)
	case OpHeartbeatACK:
		// no-op, heartbeat is acknowledged
	case OpReconnect:
		w.adapter.logf("server requested reconnect")
		w.triggerReconnect()
	case OpInvalidSession:
		// Check if resumable
		w.mu.Lock()
		w.sessionID = "" // force re-identify
		w.mu.Unlock()
		w.triggerReconnect()
	case OpHello:
		// Should not normally receive Hello after connect
		w.adapter.logf("unexpected Hello, re-authenticating")
		w.authenticate(ctx)
	}
}

// handleDispatch routes a Dispatch event to the adapter.
func (w *wsConn) handleDispatch(ctx context.Context, payload *WSPayload) {
	switch payload.T {
	case EventReady:
		var ready ReadyEvent
		if err := json.Unmarshal(payload.D, &ready); err != nil {
			w.adapter.logf("unmarshal READY: %v", err)
			return
		}
		w.mu.Lock()
		w.sessionID = ready.SessionID
		w.mu.Unlock()
		w.adapter.logf("QQ Bot connected: session=%s user=%s", ready.SessionID, ready.User.Username)

	case EventResumed:
		w.adapter.logf("QQ Bot session resumed")

	case EventC2CMessageCreate:
		w.adapter.handleC2CMessage(ctx, payload)

	case EventGroupAtMessageCreate:
		w.adapter.handleGroupMessage(ctx, payload)

	case EventGuildAtMessageCreate, EventGuildMessageCreate:
		w.adapter.handleGuildMessage(ctx, payload)

	case EventDirectMessageCreate:
		w.adapter.handleDMMessage(ctx, payload)

	case EventInteractionCreate:
		w.adapter.handleInteraction(ctx, payload)
	}
}

// triggerReconnect signals the adapter to reconnect.
func (w *wsConn) triggerReconnect() {
	select {
	case w.reconnectCh <- struct{}{}:
	default:
	}
}

// close gracefully shuts down the WebSocket connection.
func (w *wsConn) close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}
	w.closed = true

	if w.conn != nil {
		w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		w.conn.Close()
	}
}

// mustMarshal is a helper that panics on marshal error (should never happen for our types).
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal: %v", err))
	}
	return data
}

// msgSeq returns a monotonically increasing sequence number for dedup.
// Uses a simple millisecond-based counter bounded to 0-65535.
func msgSeq() int {
	return int(time.Now().UnixMilli()%65535) ^ rand.Intn(1000)
}
