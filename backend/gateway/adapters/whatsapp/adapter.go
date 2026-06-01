package whatsapp

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/insmtx/Leros/backend/gateway/pkg/core"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// Adapter implements the WhatsApp channel via a Node.js bridge process.
//
// Implements: Connector, Lifecycle, Receiver, Sender, ManagedProcess.
type Adapter struct {
	cfg     types.ChannelConfig
	process *BridgeProcess
	client  *BridgeClient
	logger  *log.Logger

	callback core.MessageCallback

	mu      sync.Mutex
	cursor  string
	running bool
}

// NewAdapter creates a WhatsApp adapter from channel config.
func NewAdapter(cfg types.ChannelConfig) *Adapter {
	scriptPath, _ := cfg.Extra["bridge_script"].(string)
	sessionPath, _ := cfg.Extra["session_path"].(string)
	port := 8099
	if p, ok := cfg.Extra["bridge_port"].(float64); ok && p > 0 {
		port = int(p)
	}

	return &Adapter{
		cfg:     cfg,
		process: NewBridgeProcess(scriptPath, port, sessionPath),
		client:  NewBridgeClient(port),
	}
}

// Info returns adapter metadata.
func (a *Adapter) Info() types.AdapterInfo {
	return types.AdapterInfo{
		Code:        "whatsapp",
		Label:       "WhatsApp",
		Description: "WhatsApp messaging via Node.js bridge (baileys)",
		Version:     "1.0.0",
		Capabilities: types.ChannelCapabilities{
			SupportsIM:    true,
			SupportsStream: false,
			NeedsLongConn:  true,
			MaxMessageLen:  4096,
		},
	}
}

// OnMessage registers the inbound message callback.
func (a *Adapter) OnMessage(callback core.MessageCallback) error {
	if a.callback != nil {
		return fmt.Errorf("whatsapp adapter: OnMessage already registered")
	}
	a.callback = callback
	return nil
}

// Connect starts the bridge process and begins polling for messages.
func (a *Adapter) Connect(ctx context.Context) error {
	if a.callback == nil {
		return fmt.Errorf("whatsapp adapter: OnMessage must be called before Connect")
	}

	if err := a.process.Start(ctx); err != nil {
		return fmt.Errorf("start bridge: %w", err)
	}

	a.mu.Lock()
	a.running = true
	a.mu.Unlock()

	go a.pollLoop(ctx)
	return nil
}

// Disconnect stops the bridge process.
func (a *Adapter) Disconnect(ctx context.Context) error {
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()

	return a.process.Stop(ctx)
}

// Health checks whether the bridge is running.
func (a *Adapter) Health(ctx context.Context) error {
	if a.process.Pid() < 0 {
		return fmt.Errorf("whatsapp bridge not running")
	}
	_, err := a.client.Health(ctx)
	return err
}

// Pid returns the bridge process PID.
func (a *Adapter) Pid() int {
	return a.process.Pid()
}

// Start starts the bridge (delegates to ManagedProcess).
func (a *Adapter) Start(ctx context.Context) error {
	return a.process.Start(ctx)
}

// Stop stops the bridge (delegates to ManagedProcess).
func (a *Adapter) Stop(ctx context.Context) error {
	return a.process.Stop(ctx)
}

// Restart restarts the bridge (delegates to ManagedProcess).
func (a *Adapter) Restart(ctx context.Context) error {
	return a.process.Restart(ctx)
}

// Send delivers a message via the bridge.
func (a *Adapter) Send(ctx context.Context, target string, msg types.OutboundMessage) error {
	req := BridgeSendRequest{
		ChatID: target,
		Text:   msg.Text,
		Markdown: msg.Markdown,
	}
	if msg.ReplyTo != nil {
		req.ReplyTo = *msg.ReplyTo
	}
	if len(msg.NotifyUserIDs) > 0 {
		req.Mentions = msg.NotifyUserIDs
	}

	resp, err := a.client.SendText(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("send failed: %s", resp.Error)
	}
	return nil
}

// SendTyping is not supported by the WhatsApp bridge.
func (a *Adapter) SendTyping(ctx context.Context, target string) error {
	return nil
}

// pollLoop continuously fetches messages from the bridge.
func (a *Adapter) pollLoop(ctx context.Context) {
	for {
		a.mu.Lock()
		running := a.running
		a.mu.Unlock()
		if !running {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		a.mu.Lock()
		cursor := a.cursor
		a.mu.Unlock()

		msgs, newCursor, hasMore, err := a.client.GetMessages(ctx, cursor)
		if err != nil {
			a.logf("poll error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		a.mu.Lock()
		a.cursor = newCursor
		a.mu.Unlock()

		for _, msg := range msgs {
			env := a.normalize(&msg)
			if env == nil {
				continue
			}
			a.dispatch(ctx, env)
		}

		if !hasMore {
			time.Sleep(bridgePollInterval)
		}
	}
}

// normalize converts a bridge message to a MessageEnvelope.
func (a *Adapter) normalize(msg *BridgeMessage) *types.MessageEnvelope {
	chatType := types.ChatTypeDM
	if msg.ChatType == "group" {
		chatType = types.ChatTypeGroup
	}

	var attachments []types.Attachment
	for _, att := range msg.Attachments {
		attachments = append(attachments, types.Attachment{
			URL:      att.URL,
			MimeType: att.MimeType,
			Size:     att.Size,
		})
	}

	return &types.MessageEnvelope{
		MessageID:   msg.ID,
		TraceID:     uuid.New().String(),
		Channel:     "whatsapp",
		MessageType: types.MessageTypeText,
		SessionKey: types.SessionKey{
			Channel:  "whatsapp",
			ChatType: chatType,
			ChatID:   msg.ChatID,
			UserID:   msg.From,
		},
		Sender: types.SenderInfo{
			UserID:   msg.From,
			Username: msg.FromName,
		},
		Content: types.MessageContent{
			Text: msg.Text,
		},
		ReplyTo:     stringPtr(msg.Quote.ID),
		Attachments: attachments,
		RawEvent:    marshalJSON(msg),
		ReceivedAt:  time.Unix(msg.Timestamp, 0),
	}
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (a *Adapter) dispatch(ctx context.Context, env *types.MessageEnvelope) {
	if a.callback == nil {
		return
	}
	if err := a.callback(ctx, env); err != nil {
		a.logf("dispatch error: %v", err)
	}
}

func (a *Adapter) logf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(format, args...)
	} else {
		log.Printf("[whatsapp] "+format, args...)
	}
}
