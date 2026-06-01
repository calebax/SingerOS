package feishu

import (
	"context"
	"encoding/json"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// Send delivers an outbound message to a Feishu chat.
func (a *Adapter) Send(ctx context.Context, target string, msg types.OutboundMessage) error {
	// Build Feishu message content
	content := a.buildOutboundContent(msg)

	// Send via IM API
	_, err := a.client.SendMessage(ctx, target, "text", content)
	return err
}

// SendTyping sends a typing indicator (not supported by Feishu REST API directly).
func (a *Adapter) SendTyping(ctx context.Context, target string) error {
	// Feishu does not support typing indicators via REST API.
	// Bot reactions can be used instead if needed.
	return nil
}

// buildOutboundContent converts a gateway OutboundMessage to Feishu text JSON content.
func (a *Adapter) buildOutboundContent(msg types.OutboundMessage) string {
	text := msg.Text
	if msg.Markdown != "" {
		// Feishu does not support markdown in text messages.
		// For markdown-like formatting, use post message type.
		// Fallback: strip markdown and send as plain text.
		text = msg.Markdown
	}

	content := OutboundTextContent{Text: text}
	data, _ := json.Marshal(content)
	return string(data)
}
