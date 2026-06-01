// Package core defines the adapter interface hierarchy and channel registration system.
//
// The interface hierarchy follows the Interface Segregation Principle:
// a channel adapter implements only the interfaces that match its capabilities.
//
//	Connector        — minimal, every adapter implements this
//	Lifecycle        — for channels needing Connect/Disconnect (IM, WebSocket)
//	Receiver         — for channels that consume inbound messages
//	Sender           — for channels that send outbound messages
//	WebhookReceiver  — for channels receiving HTTP webhook events
//	ManagedProcess   — for channels that manage external processes (bridges)
//	RuntimeStatus    — additional runtime status beyond Health()
//
// Adapters declare their capabilities via AdapterInfo.Capabilities, which the
// gateway uses to decide how to interact with each channel at runtime.
package core

import (
	"context"
	"net/http"
	"time"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// Connector is the base interface every channel adapter must implement.
// It provides identity metadata so the gateway can discover and introspect the adapter.
type Connector interface {
	Info() types.AdapterInfo
}

// Lifecycle manages the connection lifecycle of a channel.
type Lifecycle interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Health(ctx context.Context) error
}

// Receiver is implemented by adapters that consume inbound messages.
type Receiver interface {
	OnMessage(callback MessageCallback) error
}

// Sender is implemented by adapters that can send outbound messages to the platform.
type Sender interface {
	Send(ctx context.Context, target string, msg types.OutboundMessage) error
	SendTyping(ctx context.Context, target string) error
}

// WebhookReceiver is implemented by adapters that receive events via HTTP webhooks.
//
// The adapter registers its own HTTP routes (e.g., /webhooks/feishu) and handles
// signature verification, payload parsing, and normalization internally. The
// normalized MessageEnvelope is passed to the gateway via Receiver.OnMessage.
type WebhookReceiver interface {
	RegisterWebhookRoutes(mux *http.ServeMux) error
}

// ManagedProcess is implemented by adapters that manage an external child process
// (e.g., a Node.js WhatsApp bridge).
type ManagedProcess interface {
	Pid() int
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
}

// StatusDetail provides richer runtime status for monitoring.
type StatusDetail struct {
	Connected      bool          `json:"connected"`
	SessionID      string        `json:"session_id,omitempty"`
	Pid            int           `json:"pid,omitempty"`
	Uptime         time.Duration `json:"uptime"`
	ReconnectCount int           `json:"reconnect_count"`
	LastError      string        `json:"last_error,omitempty"`
}

// MessageCallback is the function signature for inbound message delivery.
type MessageCallback func(ctx context.Context, envelope *types.MessageEnvelope) error

// Adapter is the composed interface for a fully capable channel.
type Adapter interface {
	Connector
	Lifecycle
	Receiver
	Sender
}
