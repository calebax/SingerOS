package qqbot

import (
	"context"
	"fmt"
	"strings"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// Send delivers a message to a QQ chat.
//
// The target is the chat identifier, which varies by chat type:
//   - C2C: user openid
//   - Group: group openid
//   - Guild: channel_id (from SessionKey.Extra["qq_channel_id"])
//   - DM: guild_id
func (a *Adapter) Send(ctx context.Context, target string, msg types.OutboundMessage) error {
	// Build the appropriate QQ message body
	body := a.buildOutboundMessage(msg)

	var err error
	// Route by chat type. The SessionKey is not available here — we use
	// the first DeliveryTarget or infer from the target format.
	if msg.DeliveryTargets != nil && len(msg.DeliveryTargets) > 0 {
		for _, dt := range msg.DeliveryTargets {
			if e := a.sendToChat(ctx, dt.ChatID, body, dt.Extra); e != nil {
				err = e
			}
		}
		return err
	}

	// Single target: try C2C first (open_id), fallback to group, then channel
	if e := a.sendToChat(ctx, target, body, nil); e != nil {
		return e
	}
	return nil
}

// sendToChat sends to a specific chat based on extra context.
func (a *Adapter) sendToChat(ctx context.Context, target string, body any, extra map[string]any) error {
	chatType := ChatTypeC2C
	if extra != nil {
		if ct, ok := extra["qq_chat_type"].(string); ok {
			chatType = ct
		}
	}

	switch chatType {
	case ChatTypeGroup:
		_, err := a.client.SendGroupMessage(ctx, target, body)
		return err
	case ChatTypeGuild:
		_, err := a.client.SendChannelMessage(ctx, target, body)
		return err
	case ChatTypeDM:
		_, err := a.client.SendDMMessage(ctx, target, body)
		return err
	default: // C2C
		_, err := a.client.SendC2CMessage(ctx, target, body)
		return err
	}
}

// SendTyping sends a typing indicator (input_notify) to the target.
func (a *Adapter) SendTyping(ctx context.Context, target string) error {
	// QQ Bot typing indicator: POST with msg_type=6 (not available via official API v2 for all contexts).
	// We send a lightweight typing notification via the C2C endpoint.
	typingBody := OutboundTextMessage{
		Content: "",
		MsgType:  6, // typing indicator
		MsgSeq:   msgSeq(),
	}
	_, err := a.client.SendC2CMessage(ctx, target, typingBody)
	return err
}

// buildOutboundMessage converts a gateway OutboundMessage to a QQ Bot API message.
func (a *Adapter) buildOutboundMessage(msg types.OutboundMessage) any {
	text := msg.Text
	if msg.Markdown != "" {
		text = msg.Markdown
	}

	// If there's markdown, send as msg_type 2 (Markdown)
	if msg.Markdown != "" && len(msg.ImageURLs) == 0 {
		return OutboundMarkdownMessage{
			Markdown: &MarkdownContent{Content: a.truncateText(msg.Markdown)},
			MsgType:  MsgTypeMarkdown,
			MsgSeq:   msgSeq(),
		}
	}

	// If there are images, send as text + image URLs (QQ needs separate upload)
	if len(msg.ImageURLs) > 0 {
		// For simplicity, send the text part first, then images
		// In production, use the media upload API
		return OutboundTextMessage{
			Content: a.truncateText(text + "\n" + strings.Join(msg.ImageURLs, "\n")),
			MsgType: MsgTypeText,
			MsgSeq:  msgSeq(),
		}
	}

	// Plain text
	return OutboundTextMessage{
		Content: a.truncateText(text),
		MsgType: MsgTypeText,
		MsgSeq:  msgSeq(),
	}
}

// truncateText truncates text to QQ's message length limit.
func (a *Adapter) truncateText(text string) string {
	if len(text) <= MaxMessageLength {
		return text
	}
	return text[:MaxMessageLength-3] + "..."
}

// SendMedia sends a media file to a chat.
func (a *Adapter) SendMedia(ctx context.Context, target string, chatType string, mediaType int, fileURL string) error {
	resp, err := a.client.UploadMedia(ctx, chatType, target, mediaType, fileURL)
	if err != nil {
		return fmt.Errorf("upload media: %w", err)
	}

	mediaMsg := OutboundMediaMessage{
		MsgType: MsgTypeMedia,
		Media:   &MediaInfo{FileInfo: resp.FileInfo},
		MsgSeq:  msgSeq(),
	}

	switch chatType {
	case ChatTypeGroup:
		_, err = a.client.SendGroupMessage(ctx, target, mediaMsg)
	case ChatTypeC2C:
		_, err = a.client.SendC2CMessage(ctx, target, mediaMsg)
	default:
		return fmt.Errorf("unsupported chat type for media: %s", chatType)
	}

	return err
}
