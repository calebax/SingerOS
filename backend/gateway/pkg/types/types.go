// Package types defines the core shared types used across the message gateway.
//
// All channels produce and consume these types, ensuring a unified contract
// regardless of the underlying platform protocol.
package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ChannelCode uniquely identifies a message channel (e.g., "feishu", "github").
type ChannelCode string

// MessageType classifies the kind of inbound message.
type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeFile  MessageType = "file"
	MessageTypeAudio MessageType = "audio"
	MessageTypeVideo MessageType = "video"
	MessageTypeEvent MessageType = "event" // webhook events (PR, push, issue, etc.)
)

// ChatType classifies the conversation context.
type ChatType string

const (
	ChatTypeDM      ChatType = "dm"
	ChatTypeGroup   ChatType = "group"
	ChatTypeChannel ChatType = "channel" // Slack/Discord-style public channel
)

// SessionKey identifies a unique conversation session across platforms.
//
// The canonical session key format is: {channel}:{chat_type}:{chat_id}:{user_id}
type SessionKey struct {
	Channel  ChannelCode `json:"channel"`
	ChatType ChatType    `json:"chat_type"`
	ChatID   string      `json:"chat_id"`
	UserID   string      `json:"user_id"`
	ThreadID string      `json:"thread_id,omitempty"`
}

// String returns the canonical session key representation.
func (s SessionKey) String() string {
	key := string(s.Channel) + ":" + string(s.ChatType) + ":" + s.ChatID + ":" + s.UserID
	if s.ThreadID != "" {
		key += ":" + s.ThreadID
	}
	return key
}

// SenderInfo describes the user who sent a message.
type SenderInfo struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	IsBot    bool   `json:"is_bot,omitempty"`
}

// MessageContent holds the content of an inbound message.
//
// For text messages, Text is the primary field.
// For media messages, MediaURLs contains the attachment URLs and Text may be the caption.
type MessageContent struct {
	Text      string   `json:"text,omitempty"`
	MediaURLs []string `json:"media_urls,omitempty"`
	// Extra holds platform-specific fields that do not fit the normalized schema.
	Extra map[string]any `json:"extra,omitempty"`
}

// MessageEnvelope is the normalized inbound message produced by every channel adapter.
//
// It is the single type that all downstream components (auth, dispatch, orchestration)
// consume, regardless of the originating platform.
type MessageEnvelope struct {
	MessageID   string         `json:"message_id"`
	TraceID     string         `json:"trace_id"`
	Channel     ChannelCode    `json:"channel"`
	MessageType MessageType    `json:"message_type"`
	SessionKey  SessionKey     `json:"session_key"`
	Sender      SenderInfo     `json:"sender"`
	Content     MessageContent `json:"content"`
	// ReplyTo is the original message ID being replied to, if any.
	ReplyTo *string `json:"reply_to,omitempty"`
	// SkillHint suggests a skill for the agent to invoke (e.g., "code_review").
	SkillHint *string `json:"skill_hint,omitempty"`
	// ChannelPrompt is an optional system prompt fragment injected by the channel.
	ChannelPrompt *string `json:"channel_prompt,omitempty"`
	// RawEvent preserves the original platform event for custom processing.
	RawEvent json.RawMessage `json:"raw_event,omitempty"`
	// Attachments contains normalized media attachments (images, files, audio, video).
	Attachments []Attachment `json:"attachments,omitempty"`
	// Mentions lists users @-mentioned in the message.
	Mentions []Mention `json:"mentions,omitempty"`
	// CommandHint marks the message as a command/button interaction.
	CommandHint *string `json:"command_hint,omitempty"`
	// TransportMeta holds transport-layer metadata (context token, cursor, platform msg id).
	TransportMeta *TransportMeta `json:"transport_meta,omitempty"`
	// CapabilitiesHint informs downstream consumers of platform capabilities.
	CapabilitiesHint *CapabilitiesHint `json:"capabilities_hint,omitempty"`
	ReceivedAt       time.Time         `json:"received_at"`
}

// Attachment represents a media attachment in an inbound message.
type Attachment struct {
	// URL is the remote URL of the attachment.
	URL string `json:"url,omitempty"`
	// LocalPath is the local cached file path, if downloaded.
	LocalPath string `json:"local_path,omitempty"`
	// PlatformID is the platform's internal media identifier.
	PlatformID string `json:"platform_id,omitempty"`
	// MimeType is the content type (e.g., "image/jpeg").
	MimeType string `json:"mime_type,omitempty"`
	// Size is the file size in bytes.
	Size int64 `json:"size,omitempty"`
	// Width and Height are image dimensions.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	// Encrypted indicates whether the attachment is encrypted at rest.
	Encrypted bool `json:"encrypted,omitempty"`
	// EncryptMeta holds platform-specific decryption parameters.
	EncryptMeta map[string]string `json:"encrypt_meta,omitempty"`
}

// Mention represents an @-mention of a user in a message.
type Mention struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	// Offset and Length indicate the position of the mention in the original text.
	Offset int `json:"offset,omitempty"`
	Length int `json:"length,omitempty"`
}

// TransportMeta holds metadata needed for reply routing across transport layers.
type TransportMeta struct {
	// ContextToken is the opaque token required to reply in the same context (WeChat, Feishu).
	ContextToken string `json:"context_token,omitempty"`
	// Cursor is the long-poll cursor for continuity.
	Cursor string `json:"cursor,omitempty"`
	// PlatformMsgID is the original platform-native message identifier.
	PlatformMsgID string `json:"platform_msg_id,omitempty"`
	// ThreadRootID is the root message ID of a thread conversation.
	ThreadRootID string `json:"thread_root_id,omitempty"`
	// ReplyMeta holds platform-specific reply routing fields.
	ReplyMeta map[string]string `json:"reply_meta,omitempty"`
}

// CapabilitiesHint informs downstream consumers about platform capabilities.
type CapabilitiesHint struct {
	// SupportsMarkdown indicates whether markdown formatting is supported.
	SupportsMarkdown bool `json:"supports_markdown,omitempty"`
	// MaxMessageLen is the platform's message length limit.
	MaxMessageLen int `json:"max_message_len,omitempty"`
	// SupportsMedia indicates which media types can be sent.
	SupportsMedia []string `json:"supports_media,omitempty"`
}

// OutboundMessage is a message to be sent to an external platform.
type OutboundMessage struct {
	Text      string   `json:"text,omitempty"`
	Markdown  string   `json:"markdown,omitempty"`
	ImageURLs []string `json:"image_urls,omitempty"`
	FilePaths []string `json:"file_paths,omitempty"`
	ReplyTo   *string  `json:"reply_to,omitempty"`
	// DeliveryTargets overrides the destination (default: reply to source).
	DeliveryTargets []DeliveryTarget `json:"delivery_targets,omitempty"`
	// NotifyUserIDs specifies which users to @mention or notify in a group.
	NotifyUserIDs []string `json:"notify_user_ids,omitempty"`
}

// DeliveryTarget specifies where an outbound message should be routed.
//
// Targets are parsed from strings following the pattern:
//
//	"origin"                     → reply to the source session
//	"{channel}:{chat_id}"        → send to a specific chat
//	"{channel}:{chat_id}:{thread}" → send to a specific thread
type DeliveryTarget struct {
	Channel  ChannelCode    `json:"channel"`
	ChatID   string         `json:"chat_id"`
	ThreadID string         `json:"thread_id,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}

// String returns the canonical delivery target representation.
func (d DeliveryTarget) String() string {
	s := string(d.Channel) + ":" + d.ChatID
	if d.ThreadID != "" {
		s += ":" + d.ThreadID
	}
	return s
}

// ParseDeliveryTarget parses a target string into a DeliveryTarget.
//
// Supported formats:
//   - "origin" → target destined for the original source session
//   - "local"  → target for local file storage
//   - "feishu:oc_xxx" → DeliveryTarget{Channel: "feishu", ChatID: "oc_xxx"}
//   - "feishu:oc_xxx:thread_123" → DeliveryTarget{Channel: "feishu", ChatID: "oc_xxx", ThreadID: "thread_123"}
func ParseDeliveryTarget(raw string) (DeliveryTarget, error) {
	switch raw {
	case "", "origin":
		return DeliveryTarget{}, nil // caller handles origin routing
	case "local":
		return DeliveryTarget{Channel: "local"}, nil
	}

	parts := strings.SplitN(raw, ":", 3)
	switch len(parts) {
	case 2:
		return DeliveryTarget{Channel: ChannelCode(parts[0]), ChatID: parts[1]}, nil
	case 3:
		return DeliveryTarget{Channel: ChannelCode(parts[0]), ChatID: parts[1], ThreadID: parts[2]}, nil
	default:
		return DeliveryTarget{}, fmt.Errorf("invalid delivery target: %s (expected channel:chat_id or channel:chat_id:thread)", raw)
	}
}

// ChannelCapabilities declares what a channel adapter can do.
//
// This metadata drives gateway behavior — e.g., whether to open a long-lived
// connection for an IM channel, or whether streaming responses are supported.
type ChannelCapabilities struct {
	// SupportsIM indicates the channel handles bidirectional instant messaging.
	SupportsIM bool `json:"supports_im"`
	// SupportsWebhook indicates the channel receives events via HTTP webhooks.
	SupportsWebhook bool `json:"supports_webhook"`
	// SupportsStream indicates the channel can deliver token-by-token streaming responses.
	SupportsStream bool `json:"supports_stream"`
	// NeedsLongConn indicates the channel requires a persistent connection (WebSocket, long-poll).
	NeedsLongConn bool `json:"needs_long_conn"`
	// MaxMessageLen is the maximum character count for a single outbound message.
	MaxMessageLen int `json:"max_message_len"`
	// MaxMediaSize is the maximum byte size for a media attachment, 0 means no limit.
	MaxMediaSize int64 `json:"max_media_size,omitempty"`
}

// AdapterInfo provides metadata about a channel adapter.
type AdapterInfo struct {
	Code         ChannelCode         `json:"code"`
	Label        string              `json:"label"`
	Description  string              `json:"description"`
	Version      string              `json:"version"`
	Capabilities ChannelCapabilities `json:"capabilities"`
}

// ChannelConfig is the base configuration every channel adapter receives.
//
// Platform-specific settings are injected via the Extra field.
type ChannelConfig struct {
	Code     ChannelCode    `json:"code"`
	Disabled bool           `json:"disabled"`
	Extra    map[string]any `json:"extra,omitempty"`
}
