package qqbot

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// WSPayload is the top-level WebSocket message frame for QQ Bot Gateway.
type WSPayload struct {
	Op int              `json:"op"`         // opcode
	D  json.RawMessage  `json:"d,omitempty"` // event data
	S  *int             `json:"s,omitempty"` // sequence number
	T  string           `json:"t,omitempty"` // event type (for op=0)
	ID string           `json:"id,omitempty"` // event id
}

// HelloData is the payload for OpCode 10 Hello.
type HelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"` // milliseconds
}

// IdentifyData is the payload for OpCode 2 Identify.
type IdentifyData struct {
	Token      string           `json:"token"`
	Intents    int              `json:"intents"`
	Shard      [2]int           `json:"shard"`
	Properties IdentifyProperties `json:"properties"`
}

// IdentifyProperties describes the client for telemetry.
type IdentifyProperties struct {
	OS      string `json:"$os"`
	Browser string `json:"$browser"`
	Device  string `json:"$device"`
}

// ResumeData is the payload for OpCode 6 Resume.
type ResumeData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int    `json:"seq"`
}

// ReadyEvent is the payload for the READY dispatch event.
type ReadyEvent struct {
	Version   int         `json:"version"`
	SessionID string      `json:"session_id"`
	User      ReadyUser   `json:"user"`
	Shard     [2]int      `json:"shard"`
}

// ReadyUser is the bot's own user info from READY.
type ReadyUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

// HeartbeatData is the payload for OpCode 1 Heartbeat.
type HeartbeatData struct {
	Seq *int `json:"d"` // last received sequence number, null for first heartbeat
}

// --- Token API Types ---

// TokenRequest is the request body for getting an access token.
type TokenRequest struct {
	AppID        string `json:"appId"`
	ClientSecret string `json:"clientSecret"`
}

// FlexInt handles JSON fields that may be string or number.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	var num int
	if err := json.Unmarshal(data, &num); err == nil {
		*f = FlexInt(num)
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("flex_int: cannot unmarshal %s as int or string", string(data))
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		return fmt.Errorf("flex_int: cannot parse %q: %w", str, err)
	}
	*f = FlexInt(v)
	return nil
}

// TokenResponse is the response from the token endpoint.
type TokenResponse struct {
	AccessToken string  `json:"access_token"`
	ExpiresIn   FlexInt `json:"expires_in"` // seconds (QQ returns string or number)
}

// --- Gateway URL API ---

// GatewayResponse is the response from the gateway URL endpoint.
type GatewayResponse struct {
	URL               string              `json:"url"`
	Shards            int                 `json:"shards"`
	SessionStartLimit SessionStartLimit   `json:"session_start_limit"`
}

// SessionStartLimit describes rate limits for new connections.
type SessionStartLimit struct {
	Total          int `json:"total"`
	Remaining      int `json:"remaining"`
	ResetAfter     int `json:"reset_after"`     // milliseconds
	MaxConcurrency int `json:"max_concurrency"`
}

// --- Message Event Types ---

// C2CMessageEvent is the payload for C2C_MESSAGE_CREATE.
type C2CMessageEvent struct {
	ID        string        `json:"id"`
	Author    MessageAuthor `json:"author"`
	Content   string        `json:"content"`
	Timestamp string        `json:"timestamp"`
}

// GroupAtMessageEvent is the payload for GROUP_AT_MESSAGE_CREATE.
type GroupAtMessageEvent struct {
	ID           string        `json:"id"`
	Author       MessageAuthor `json:"author"`
	Content      string        `json:"content"`
	Timestamp    string        `json:"timestamp"`
	GroupOpenID  string        `json:"group_openid"`
	GroupID      string        `json:"group_id"`
	MemberOpenID string        `json:"member_openid"`
}

// GuildAtMessageEvent is the payload for AT_MESSAGE_CREATE (guild channel).
type GuildAtMessageEvent struct {
	ID              string          `json:"id"`
	Author          GuildMemberUser `json:"author"`
	Content         string          `json:"content"`
	Timestamp       string          `json:"timestamp"`
	ChannelID       string          `json:"channel_id"`
	GuildID         string          `json:"guild_id"`
	Attachments     []MessageAttachment `json:"attachments,omitempty"`
	MessageReference *MessageReference  `json:"message_reference,omitempty"`
}

// DirectMessageEvent is the payload for DIRECT_MESSAGE_CREATE.
type DirectMessageEvent struct {
	ID              string          `json:"id"`
	Author          GuildMemberUser `json:"author"`
	Content         string          `json:"content"`
	Timestamp       string          `json:"timestamp"`
	GuildID         string          `json:"guild_id"`
	MemberOpenID    string          `json:"member_openid"`
	Attachments     []MessageAttachment `json:"attachments,omitempty"`
	MessageReference *MessageReference  `json:"message_reference,omitempty"`
}

// InteractionEvent is the payload for INTERACTION_CREATE (button callback).
type InteractionEvent struct {
	ID            string              `json:"id"`
	ApplicationID string              `json:"application_id"`
	Type          int                 `json:"type"`
	Data          InteractionData     `json:"data"`
	GuildID       string              `json:"guild_id,omitempty"`
	ChannelID     string              `json:"channel_id,omitempty"`
	GroupOpenID   string              `json:"group_openid,omitempty"`
	ChatType      int                 `json:"chat_type"`
	Timestamp     string              `json:"timestamp"`
	Member        *GuildMemberUser    `json:"member,omitempty"`
	User          *GuildMemberUser    `json:"user,omitempty"`
	Version       int                 `json:"version"`
	Scene         string              `json:"scene"`
}

// InteractionData contains the interaction-specific payload.
type InteractionData struct {
	Type     int                  `json:"type"`
	Resolved json.RawMessage      `json:"resolved,omitempty"`
	ButtonID string               `json:"button_id,omitempty"`
	ButtonData string             `json:"button_data,omitempty"`
	Feature  *InteractionFeature  `json:"feature,omitempty"`
}

// InteractionFeature describes the interaction context.
type InteractionFeature struct {
	Type  int    `json:"type"`
	MsgID string `json:"msg_id,omitempty"`
}

// MessageAuthor is the author info for C2C and group messages.
type MessageAuthor struct {
	ID          string `json:"id"`
	UserOpenID  string `json:"user_openid"`
	MemberOpenID string `json:"member_openid,omitempty"`
	UnionOpenID string `json:"union_openid,omitempty"`
	UnionUserAccount string `json:"union_user_account,omitempty"`
}

// GuildMemberUser is the user info for guild messages.
type GuildMemberUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar,omitempty"`
	Bot      bool   `json:"bot,omitempty"`
}

// MessageAttachment represents an attachment in a guild message.
type MessageAttachment struct {
	URL          string `json:"url"`
	ContentType  string `json:"content_type,omitempty"`
	Filename     string `json:"filename,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
}

// MessageReference represents a quoted/replied message reference.
type MessageReference struct {
	MessageID             string `json:"message_id"`
	IgnoreGetMessageError bool   `json:"ignore_get_message_error,omitempty"`
}

// --- Outbound Message Types ---

// OutboundTextMessage is a text message for REST API.
type OutboundTextMessage struct {
	Content  string       `json:"content"`
	MsgType  int          `json:"msg_type"`
	MsgID    string       `json:"msg_id,omitempty"`    // reply target
	EventID  string       `json:"event_id,omitempty"`  // reply to event
	MsgSeq   int          `json:"msg_seq,omitempty"`   // dedup sequence
	Keyboard *InlineKeyboard `json:"keyboard,omitempty"`
}

// OutboundMarkdownMessage is a markdown message for REST API.
type OutboundMarkdownMessage struct {
	Markdown *MarkdownContent `json:"markdown,omitempty"`
	MsgType  int              `json:"msg_type"`
	MsgID    string           `json:"msg_id,omitempty"`
	EventID  string           `json:"event_id,omitempty"`
	MsgSeq   int              `json:"msg_seq,omitempty"`
	Keyboard *InlineKeyboard   `json:"keyboard,omitempty"`
}

// MarkdownContent holds the markdown parameters.
type MarkdownContent struct {
	Content string `json:"content"`
}

// OutboundMediaMessage is a media message for REST API.
type OutboundMediaMessage struct {
	MsgType int         `json:"msg_type"` // always 7
	Media   *MediaInfo  `json:"media"`
	MsgID   string      `json:"msg_id,omitempty"`
	MsgSeq  int         `json:"msg_seq,omitempty"`
}

// MediaInfo holds the uploaded file info for outbound media.
type MediaInfo struct {
	FileInfo string `json:"file_info"`
}

// SendMessageResponse is the response from sending a message.
type SendMessageResponse struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Code      int    `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// --- Media Upload Types ---

// UploadMediaResponse is the response from uploading media.
type UploadMediaResponse struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int    `json:"ttl"`
	ID       string `json:"id"`
}

// --- Inline Keyboard Types ---

// InlineKeyboard represents a row-based inline keyboard.
type InlineKeyboard struct {
	Content *KeyboardContent `json:"content,omitempty"`
	ID      string            `json:"id,omitempty"`
	Rows    []KeyboardRow     `json:"rows"`
}

// KeyboardContent is the message content shown alongside the keyboard.
type KeyboardContent struct {
	Content string `json:"content,omitempty"`
	MsgType int    `json:"msg_type,omitempty"`
}

// KeyboardRow is a row of buttons.
type KeyboardRow struct {
	Buttons []KeyboardButton `json:"buttons"`
}

// KeyboardButton is a single button in an inline keyboard.
type KeyboardButton struct {
	ID         string              `json:"id"`
	RenderData KeyboardRenderData  `json:"render_data"`
	Action     KeyboardAction      `json:"action"`
}

// KeyboardRenderData describes the button appearance.
type KeyboardRenderData struct {
	Label        string `json:"label"`
	VisitedLabel string `json:"visited_label"`
	Style        int    `json:"style"` // 0=gray, 1=blue
}

// KeyboardAction describes what happens when the button is clicked.
type KeyboardAction struct {
	Type     int             `json:"type"` // 0=跳转, 1=回调, 2=指令
	Data     string          `json:"data"`
	Permission KeyboardPermission `json:"permission"`
	ClickLimit int           `json:"click_limit"`
	UnsupportTips string     `json:"unsupport_tips,omitempty"`
}

// KeyboardPermission specifies who can click the button.
type KeyboardPermission struct {
	Type   int      `json:"type"` // 0=指定用户, 1=仅管理者, 2=所有人, 3=指定身份组
	SpecifyUserIDs []string `json:"specify_user_ids,omitempty"`
}

// InteractionResponse is the ACK for INTERACTION_CREATE.
type InteractionResponse struct {
	Code int `json:"code"` // 0=success
}
