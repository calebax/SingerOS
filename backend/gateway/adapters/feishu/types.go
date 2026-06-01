package feishu

import "encoding/json"

// TokenResponse is the response from app_access_token/internal.
type TokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	AppAccessToken    string `json:"app_access_token"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"` // seconds
}

// BotInfo is the response from bot/v3/info.
type BotInfo struct {
	Bot struct {
		OpenID    string `json:"open_id"`
		AppID     string `json:"app_id"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	} `json:"bot"`
}

// WSGatewayResponse is the response from ws/v1/url/get.
type WSGatewayResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		URL string `json:"url"`
	} `json:"data"`
}

// --- Inbound Event Types ---

// EventHeader is the common header for all Feishu events.
type EventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
}

// WSEvent is the raw WebSocket event frame.
type WSEvent struct {
	Type string          `json:"type"` // "message" | "error" | "close"
	Data json.RawMessage `json:"data"`
}

// WSMessage is the WebSocket message received from Feishu.
type WSMessage struct {
	Schema string          `json:"schema"` // "2.0"
	Header WSEventHeader   `json:"header"`
	Event  json.RawMessage `json:"event"`
}

// WSEventHeader is the inner header in a WS message.
type WSEventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key,omitempty"`
}

// MessageEvent is the im.message.receive_v1 payload.
type MessageEvent struct {
	Sender  MessageSender  `json:"sender"`
	Message MessageContent `json:"message"`
}

// MessageSender describes the message sender.
type MessageSender struct {
	SenderID struct {
		UserID   string `json:"user_id,omitempty"`
		OpenID   string `json:"open_id,omitempty"`
		UnionID  string `json:"union_id,omitempty"`
	} `json:"sender_id"`
	SenderType string `json:"sender_type"` // "user" | "app" | "anonymous"
	TenantKey  string `json:"tenant_key,omitempty"`
}

// MessageContent holds the message body.
type MessageContent struct {
	MessageID   string          `json:"message_id"`
	RootID      string          `json:"root_id,omitempty"`
	ParentID    string          `json:"parent_id,omitempty"`
	CreateTime  string          `json:"create_time"`
	ChatID      string          `json:"chat_id"`
	ChatType    string          `json:"chat_type"`
	Content     string          `json:"content"` // JSON string, must be reparsed
	MsgType     string          `json:"msg_type"`
	Mentions    []FeishuMention `json:"mentions,omitempty"`
	UserAgent   string          `json:"user_agent,omitempty"`
}

// FeishuMention is an @-mention in a Feishu message.
type FeishuMention struct {
	Key      string `json:"key"`
	ID       struct {
		OpenID string `json:"open_id,omitempty"`
		UnionID string `json:"union_id,omitempty"`
		UserID string `json:"user_id,omitempty"`
	} `json:"id"`
	Name      string `json:"name"`
	TenantKey string `json:"tenant_key,omitempty"`
}

// TextContent is parsed from MessageContent.Content when msg_type=text.
type TextContent struct {
	Text string `json:"text"`
}

// PostContent is parsed from MessageContent.Content when msg_type=post.
type PostContent struct {
	Title   string         `json:"title,omitempty"`
	Content [][]PostElement `json:"content"`
}

// PostElement is a single element in a post message.
type PostElement struct {
	Tag      string `json:"tag"`
	Text     string `json:"text,omitempty"`
	ImageKey string `json:"image_key,omitempty"`
	FileKey  string `json:"file_key,omitempty"`
	Href     string `json:"href,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	UserName string `json:"user_name,omitempty"`
}

// ImageContent is parsed from MessageContent.Content when msg_type=image.
type ImageContent struct {
	ImageKey string `json:"image_key"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// FileContent is parsed from MessageContent.Content when msg_type=file/audio/media.
type FileContent struct {
	FileKey string `json:"file_key"`
	FileName string `json:"file_name,omitempty"`
	Duration int    `json:"duration,omitempty"`
}

// InteractiveContent is parsed from interactive/card message.
type InteractiveContent struct {
	Header struct {
		Title struct {
			Content string `json:"content"`
		} `json:"title"`
	} `json:"header"`
	Elements []struct {
		Tag   string `json:"tag"`
		Text  struct {
			Content string `json:"content"`
		} `json:"text,omitempty"`
		Actions []struct {
			Tag  string `json:"tag"`
			Text struct {
				Content string `json:"content"`
			} `json:"text"`
		} `json:"actions,omitempty"`
	} `json:"elements"`
}

// ReactionEvent is the payload for reaction created/deleted events.
type ReactionEvent struct {
	MessageID  string `json:"message_id"`
	ReactionType struct {
		EmojiType string `json:"emoji_type"`
	} `json:"reaction_type"`
	UserID  struct {
		OpenID string `json:"open_id"`
		UnionID string `json:"union_id,omitempty"`
		UserID string `json:"user_id,omitempty"`
	} `json:"user_id"`
	OperatorType string `json:"operator_type"`
	ActionTime  string `json:"action_time"`
}

// CardActionEvent is the payload for card.action.trigger.
type CardActionEvent struct {
	Operator struct {
		OpenID  string `json:"open_id"`
		UnionID string `json:"union_id,omitempty"`
		UserID  string `json:"user_id,omitempty"`
		TenantKey string `json:"tenant_key,omitempty"`
	} `json:"operator"`
	Token   string          `json:"token"`
	Action  json.RawMessage `json:"action"`
	OpenChatID string       `json:"open_chat_id"`
	OpenMessageID string    `json:"open_message_id"`
	Context struct {
		OpenMessageID string `json:"open_message_id,omitempty"`
	} `json:"context,omitempty"`
}

// --- Outbound Message Types ---

// SendMessageRequest is the request to POST /open-apis/im/v1/messages.
type SendMessageRequest struct {
	ReceiveID string      `json:"receive_id"`
	MsgType   string      `json:"msg_type"`
	Content   string      `json:"content"` // JSON string
	UUID      string      `json:"uuid,omitempty"`
}

// SendMessageResponse is the response from sending a message.
type SendMessageResponse struct {
	Code int                      `json:"code"`
	Msg  string                   `json:"msg"`
	Data SendMessageResponseData  `json:"data,omitempty"`
}

// SendMessageResponseData holds the message ID.
type SendMessageResponseData struct {
	MessageID  string `json:"message_id"`
	RootID     string `json:"root_id,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
	CreateTime string `json:"create_time"`
}

// OutboundTextContent is the JSON string for msg_type=text.
type OutboundTextContent struct {
	Text string `json:"text"`
}
