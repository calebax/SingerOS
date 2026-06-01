package whatsapp

import "encoding/json"

// BridgeHealth is the response from GET /health.
type BridgeHealth struct {
	Status  string `json:"status"` // "ok" | "error"
	Uptime  int64  `json:"uptime"` // seconds
	Version string `json:"version,omitempty"`
}

// BridgeMessage is a message received from GET /messages.
type BridgeMessage struct {
	ID         string          `json:"id"`
	ChatID     string          `json:"chat_id"`
	ChatType   string          `json:"chat_type"` // "dm" | "group"
	From       string          `json:"from"`
	FromName   string          `json:"from_name,omitempty"`
	Text       string          `json:"text,omitempty"`
	Attachments []BridgeAttachment `json:"attachments,omitempty"`
	Quote      *BridgeQuote     `json:"quote,omitempty"`
	Timestamp  int64            `json:"timestamp"`
}

// BridgeAttachment is a media attachment in a WhatsApp message.
type BridgeAttachment struct {
	Type     string `json:"type"` // "image" | "video" | "audio" | "document"
	URL      string `json:"url,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

// BridgeQuote is a quoted/replied message reference.
type BridgeQuote struct {
	ID   string `json:"id"`
	Text string `json:"text,omitempty"`
	From string `json:"from,omitempty"`
}

// BridgeSendRequest is the body for POST /send.
type BridgeSendRequest struct {
	ChatID    string          `json:"chat_id"`
	Text      string          `json:"text,omitempty"`
	Markdown  string          `json:"markdown,omitempty"`
	ReplyTo   string          `json:"reply_to,omitempty"`
	Mentions  []string        `json:"mentions,omitempty"`
}

// BridgeSendResponse is the response from POST /send.
type BridgeSendResponse struct {
	Success bool   `json:"success"`
	MsgID   string `json:"msg_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// BridgeSendMediaRequest is the body for POST /send-media.
type BridgeSendMediaRequest struct {
	ChatID   string `json:"chat_id"`
	Type     string `json:"type"` // "image" | "video" | "audio" | "document"
	URL      string `json:"url,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Caption  string `json:"caption,omitempty"`
	ReplyTo  string `json:"reply_to,omitempty"`
}

// BridgeMessagesResponse is the response from GET /messages.
type BridgeMessagesResponse struct {
	Messages []BridgeMessage `json:"messages"`
	HasMore  bool            `json:"has_more"`
	Cursor   string          `json:"cursor,omitempty"`
}

// marshalJSON is a helper.
func marshalJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
