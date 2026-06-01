package dispatch

import (
	"fmt"
	"sync"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// Policy determines whether to allow or deny a message.
type Policy int

const (
	PolicyDeny  Policy = iota // reject the message
	PolicyAllow               // accept the message
)

// Rule evaluates whether a message is permitted.
type Rule func(env *types.MessageEnvelope) Policy

// AuthGate enforces authorization policies on inbound messages.
//
// Rules are evaluated in registration order. The first non-PolicyAllow
// result terminates evaluation and is returned. A PolicyDeny from any
// rule prevents the message from being processed.
type AuthGate struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewAuthGate creates an empty authorization gate.
func NewAuthGate() *AuthGate {
	return &AuthGate{}
}

// AddRule appends a rule to the evaluation chain.
func (g *AuthGate) AddRule(rule Rule) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rules = append(g.rules, rule)
}

// PrependRule inserts a rule at the front of the chain.
func (g *AuthGate) PrependRule(rule Rule) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rules = append([]Rule{rule}, g.rules...)
}

// Authorize evaluates all rules against the message.
//
// Returns nil if the message is allowed, or an error describing why it was denied.
func (g *AuthGate) Authorize(env *types.MessageEnvelope) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for i, rule := range g.rules {
		switch rule(env) {
		case PolicyAllow:
			continue // passed this rule
		case PolicyDeny:
			return fmt.Errorf("denied by rule %d: sender=%s channel=%s", i, env.Sender.UserID, env.Channel)
		default:
			return fmt.Errorf("unknown policy at rule %d", i)
		}
	}
	return nil // all rules passed
}

// --- Built-in Rule Factories ---

// AllowlistUsers returns a rule that allows only the specified user IDs.
// All other users are denied.
func AllowlistUsers(userIDs ...string) Rule {
	set := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		set[id] = struct{}{}
	}
	return func(env *types.MessageEnvelope) Policy {
		if _, ok := set[env.Sender.UserID]; ok {
			return PolicyAllow
		}
		return PolicyDeny
	}
}

// DenylistUsers returns a rule that denies the specified user IDs.
func DenylistUsers(userIDs ...string) Rule {
	set := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		set[id] = struct{}{}
	}
	return func(env *types.MessageEnvelope) Policy {
		if _, ok := set[env.Sender.UserID]; ok {
			return PolicyDeny
		}
		return PolicyAllow
	}
}

// AllowBots returns a rule that allows or denies bot senders.
func AllowBots(allow bool) Rule {
	return func(env *types.MessageEnvelope) Policy {
		if env.Sender.IsBot == allow {
			return PolicyAllow
		}
		return PolicyDeny
	}
}

// AllowlistChats returns a rule that allows only messages from the specified
// session keys (channel:chat_type:chat_id). All other chats are denied.
func AllowlistChats(sessionKeys ...types.SessionKey) Rule {
	set := make(map[string]struct{}, len(sessionKeys))
	for _, sk := range sessionKeys {
		set[sk.String()] = struct{}{}
	}
	return func(env *types.MessageEnvelope) Policy {
		if _, ok := set[env.SessionKey.String()]; ok {
			return PolicyAllow
		}
		return PolicyDeny
	}
}

// DenylistChats returns a rule that denies messages from the specified chats.
func DenylistChats(sessionKeys ...types.SessionKey) Rule {
	set := make(map[string]struct{}, len(sessionKeys))
	for _, sk := range sessionKeys {
		set[sk.String()] = struct{}{}
	}
	return func(env *types.MessageEnvelope) Policy {
		if _, ok := set[env.SessionKey.String()]; ok {
			return PolicyDeny
		}
		return PolicyAllow
	}
}

// RequireMention returns a rule that requires the message text to contain
// the given phrase (e.g., "@bot_name") in group chats. DM messages always pass.
func RequireMention(phrase string) Rule {
	return func(env *types.MessageEnvelope) Policy {
		if env.SessionKey.ChatType != types.ChatTypeGroup {
			return PolicyAllow
		}
		if len(env.Content.Text) == 0 {
			return PolicyDeny
		}
		// simple substring check — adapters should strip @mentions when normalizing
		for i := 0; i <= len(env.Content.Text)-len(phrase); i++ {
			if env.Content.Text[i:i+len(phrase)] == phrase {
				return PolicyAllow
			}
		}
		return PolicyDeny
	}
}
