package dispatch

import (
	"context"
	"fmt"
	"sync"

	"github.com/insmtx/Leros/backend/gateway/pkg/core"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// DeliveryRouter resolves DeliveryTarget strings to channel Sender instances
// and routes outbound messages to the correct platform adapter.
//
// The router maintains a map of channel code → Sender and provides methods
// for normal message delivery and typing indicators.
type DeliveryRouter struct {
	mu      sync.RWMutex
	senders map[types.ChannelCode]core.Sender
}

// NewDeliveryRouter creates an empty delivery router.
func NewDeliveryRouter() *DeliveryRouter {
	return &DeliveryRouter{
		senders: make(map[types.ChannelCode]core.Sender),
	}
}

// Register binds a Sender to a channel code.
// This is called during gateway startup for each enabled channel that implements Sender.
func (r *DeliveryRouter) Register(code types.ChannelCode, sender core.Sender) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.senders[code] = sender
}

// Unregister removes a channel from the router.
func (r *DeliveryRouter) Unregister(code types.ChannelCode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.senders, code)
}

// Send delivers a message to the specified targets.
//
// If any target fails, the router continues to the remaining targets
// and returns a multi-error at the end.
func (r *DeliveryRouter) Send(ctx context.Context, targets []types.DeliveryTarget, msg types.OutboundMessage) error {
	var errs []error
	for _, target := range targets {
		sender, ok := r.lookup(target.Channel)
		if !ok {
			errs = append(errs, fmt.Errorf("no sender registered for channel %s", target.Channel))
			continue
		}
		if err := sender.Send(ctx, target.ChatID, msg); err != nil {
			errs = append(errs, fmt.Errorf("send to %s: %w", target, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("delivery errors: %v", errs)
	}
	return nil
}

// SendTyping sends a typing indicator to the specified channel target.
func (r *DeliveryRouter) SendTyping(ctx context.Context, channel types.ChannelCode, chatID string) error {
	sender, ok := r.lookup(channel)
	if !ok {
		return fmt.Errorf("no sender registered for channel %s", channel)
	}
	return sender.SendTyping(ctx, chatID)
}

// HasChannel reports whether a channel code is registered.
func (r *DeliveryRouter) HasChannel(code types.ChannelCode) bool {
	_, ok := r.lookup(code)
	return ok
}

func (r *DeliveryRouter) lookup(code types.ChannelCode) (core.Sender, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sender, ok := r.senders[code]
	return sender, ok
}
