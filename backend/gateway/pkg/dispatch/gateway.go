// Package dispatch contains the in-flight message dispatch infrastructure.
//
// The gateway is the central message router: inbound messages from any channel
// pass through a pipeline (dedup → auth → middleware → dispatch) before being
// published to the event bus. Outbound messages are routed back through the
// correct channel adapter's Sender implementation.
package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/insmtx/Leros/backend/gateway/pkg/core"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// MiddlewareResult controls the pipeline flow after a middleware runs.
type MiddlewareResult int

const (
	// MiddlewareContinue passes the message to the next middleware or stage.
	MiddlewareContinue MiddlewareResult = iota

	// MiddlewareDrop silently discards the message (noise, broadcast, non-target chat).
	// The message is not published to NATS and no response is sent.
	MiddlewareDrop

	// MiddlewareHandled indicates the middleware has fully processed the message
	// locally (URL verification, pairing ACK, health check). The message is not
	// forwarded to NATS but the caller considers it successfully processed.
	MiddlewareHandled
)

// InboundMiddleware is a hook in the inbound processing pipeline.
// Each middleware runs in registration order and returns:
//   - (MiddlewareContinue, nil) — proceed to next stage
//   - (MiddlewareDrop, nil)    — silently discard (no NATS, no response)
//   - (MiddlewareHandled, nil) — middleware consumed the message, stop pipeline
//   - (_, error)               — processing failed, logged but pipeline stops
type InboundMiddleware func(ctx context.Context, env *types.MessageEnvelope) (MiddlewareResult, error)

// HandleResult describes the outcome of Handle().
type HandleResult struct {
	// Published is true when the message reached the NATS event bus.
	Published bool
	// Handled is true when a middleware consumed the message locally.
	Handled bool
	// Dropped is true when the message was dropped (dedup, auth deny, middleware drop).
	Dropped bool
	// Topic is the NATS topic published to (if Published is true).
	Topic string
}

// DefaultDedupTTL is the default deduplication cache TTL.
const DefaultDedupTTL = 10 * time.Minute

// EventPublisher is the gateway's output — normalized events are published
// to the event bus for downstream consumers (orchestrator, agent runtime, etc.).
type EventPublisher interface {
	Publish(ctx context.Context, topic string, event any) error
}

// MessageGateway runs the inbound/outbound message pipelines.
//
// It is the single entry point for all channel messages, responsible for:
//   - Deduplication (prevent double-processing of the same message)
//   - Authorization (is this sender allowed?)
//   - Middleware chain (Drop/Handled/Continue short-circuit)
//   - Dispatch (publish to event bus for downstream processing)
//
// Outbound messages are routed through DeliveryRouter, which resolves
// target strings to channel Sender instances.
type MessageGateway struct {
	router      *DeliveryRouter
	publisher   EventPublisher
	authGate    *AuthGate
	middlewares []InboundMiddleware

	// deduplication
	dedupMu    sync.Mutex
	dedupCache map[string]time.Time
	dedupTTL   time.Duration
}

// GatewayOption configures the MessageGateway.
type GatewayOption func(*MessageGateway)

// WithAuthGate sets the authorization gate.
func WithAuthGate(gate *AuthGate) GatewayOption {
	return func(g *MessageGateway) {
		g.authGate = gate
	}
}

// WithDedupTTL sets the deduplication cache TTL. Default: 10 minutes.
func WithDedupTTL(ttl time.Duration) GatewayOption {
	return func(g *MessageGateway) {
		g.dedupTTL = ttl
	}
}

// WithMiddleware adds an inbound middleware hook.
func WithMiddleware(mw InboundMiddleware) GatewayOption {
	return func(g *MessageGateway) {
		g.middlewares = append(g.middlewares, mw)
	}
}

// NewMessageGateway creates a message gateway.
//
// The router is required. Publisher may be nil if the gateway is used
// only for outbound routing (e.g., in a worker process).
func NewMessageGateway(router *DeliveryRouter, publisher EventPublisher, opts ...GatewayOption) *MessageGateway {
	g := &MessageGateway{
		router:     router,
		publisher:  publisher,
		dedupCache: make(map[string]time.Time),
		dedupTTL:   10 * time.Minute,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Handle is the main inbound entry point. Every channel adapter calls this
// for each message it receives.
//
// The pipeline is:
//  1. Dedup (skip if already seen) → Dropped=true
//  2. Middleware chain → Drop/Handled/Continue short-circuit
//  3. Auth check → deny drops
//  4. Publish to event bus → Published=true
func (g *MessageGateway) Handle(ctx context.Context, env *types.MessageEnvelope) (*HandleResult, error) {
	if env == nil {
		return nil, fmt.Errorf("message envelope is nil")
	}
	result := &HandleResult{}

	// 1. Deduplication
	if g.isDuplicate(env.MessageID) {
		result.Dropped = true
		return result, nil
	}
	g.markSeen(env.MessageID)

	// 2. Middleware chain
	if action, err := g.runMiddleware(ctx, env); err != nil {
		return result, fmt.Errorf("middleware: %w", err)
	} else if action != nil {
		return action, nil
	}

	// 3. Authorization
	if g.authGate != nil {
		if err := g.authGate.Authorize(env); err != nil {
			result.Dropped = true
			return result, nil
		}
	}

	// 4. Publish to event bus
	if g.publisher != nil {
		topic := g.topicFor(env)
		if err := g.publisher.Publish(ctx, topic, env); err != nil {
			return result, fmt.Errorf("publish to %s: %w", topic, err)
		}
		result.Published = true
		result.Topic = topic
	}

	return result, nil
}

// runMiddleware iterates the middleware chain. The first non-Continue result
// wins and short-circuits the rest of the pipeline.
func (g *MessageGateway) runMiddleware(ctx context.Context, env *types.MessageEnvelope) (*HandleResult, error) {
	for _, mw := range g.middlewares {
		action, err := mw(ctx, env)
		if err != nil {
			return nil, err
		}
		switch action {
		case MiddlewareContinue:
			continue
		case MiddlewareDrop:
			result := &HandleResult{Dropped: true}
			return result, nil
		case MiddlewareHandled:
			result := &HandleResult{Handled: true}
			return result, nil
		}
	}
	return nil, nil
}

// Send routes an outbound message to the appropriate channel Sender.
//
// If msg.DeliveryTargets is set, those targets are used. Otherwise, the
// source SessionKey is used as the reply target.
func (g *MessageGateway) Send(ctx context.Context, source types.SessionKey, msg types.OutboundMessage) error {
	targets := msg.DeliveryTargets
	if len(targets) == 0 {
		targets = []types.DeliveryTarget{
			{
				Channel:  source.Channel,
				ChatID:   source.ChatID,
				ThreadID: source.ThreadID,
			},
		}
	}
	return g.router.Send(ctx, targets, msg)
}

// SendTyping sends a typing indicator back to the source channel.
func (g *MessageGateway) SendTyping(ctx context.Context, source types.SessionKey) error {
	return g.router.SendTyping(ctx, source.Channel, source.ChatID)
}

// Router returns the delivery router for direct access.
func (g *MessageGateway) Router() *DeliveryRouter {
	return g.router
}

// topicFor resolves the NATS topic for an inbound message.
func (g *MessageGateway) topicFor(env *types.MessageEnvelope) string {
	return fmt.Sprintf("interaction.%s.%s", env.Channel, env.MessageType)
}

// isDuplicate checks whether a message ID was recently processed.
func (g *MessageGateway) isDuplicate(msgID string) bool {
	if msgID == "" {
		return false
	}
	g.dedupMu.Lock()
	defer g.dedupMu.Unlock()

	g.evictExpired()

	_, exists := g.dedupCache[msgID]
	return exists
}

// markSeen records a message ID in the deduplication cache.
func (g *MessageGateway) markSeen(msgID string) {
	if msgID == "" {
		return
	}
	g.dedupMu.Lock()
	defer g.dedupMu.Unlock()

	g.evictExpired()
	g.dedupCache[msgID] = time.Now()
}

// evictExpired removes stale entries from the dedup cache.
// Must be called while holding dedupMu.
func (g *MessageGateway) evictExpired() {
	cutoff := time.Now().Add(-g.dedupTTL)
	for id, seen := range g.dedupCache {
		if seen.Before(cutoff) {
			delete(g.dedupCache, id)
		}
	}
}

// dedupHash returns a short hash of the input for compact storage.
func dedupHash(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:8])
}

// SetAuthGate replaces the auth gate at runtime.
func (g *MessageGateway) SetAuthGate(gate *AuthGate) {
	g.authGate = gate
}

// AdapterCallback returns a core.MessageCallback that feeds into this gateway.
//
// Use this to wire an adapter to the gateway:
//
//	adapter.OnMessage(gw.AdapterCallback(adapterCode))
func (g *MessageGateway) AdapterCallback(code types.ChannelCode) core.MessageCallback {
	return func(ctx context.Context, env *types.MessageEnvelope) error {
		if env.Channel == "" {
			env.Channel = code
		}
		result, err := g.Handle(ctx, env)
		if err != nil {
			return err
		}
		_ = result // result contains Published/Handled/Dropped for metrics
		return nil
	}
}
