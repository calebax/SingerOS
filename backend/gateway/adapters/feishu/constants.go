package feishu

// Feishu/Lark API base URLs.
const (
	APIBaseURL   = "https://open.feishu.cn" // default; can be overridden by config domain
	LarkBaseURL  = "https://open.larksuite.com"
)

// Token endpoints.
const (
	tokenAppAccessPath = "/open-apis/auth/v3/app_access_token/internal"
	botInfoPath        = "/open-apis/bot/v3/info"
)

// Message endpoints.
const (
	sendMessagePath    = "/open-apis/im/v1/messages"
	replyMessagePath   = "/open-apis/im/v1/messages/%s/reply"
	uploadImagePath    = "/open-apis/im/v1/images"
	uploadFileDirPath  = "/open-apis/im/v1/files"
	messageReadPath    = "/open-apis/im/v1/messages/%s/read_users"
)

// WebSocket endpoints.
const (
	wsGatewayPath = "/open-apis/ws/v1/url/get"
)

// Message types (inbound).
const (
	MsgTypeText        = "text"
	MsgTypePost        = "post"
	MsgTypeImage       = "image"
	MsgTypeFile        = "file"
	MsgTypeAudio       = "audio"
	MsgTypeMedia       = "media"
	MsgTypeInteractive = "interactive"
	MsgTypeShareChat   = "share_chat"
	MsgTypeMergeForward = "merge_forward"
)

// Chat types.
const (
	ChatTypeC2C   = "c2c"
	ChatTypeGroup = "group"
	ChatTypeP2P   = "p2p" // Feishu bot-personal
)

// Event types (WebSocket dispatch / Webhook header).
const (
	EventMessageReceived  = "im.message.receive_v1"
	EventMessageRead      = "im.message.read_v1"
	EventMessageReaction  = "im.message.reaction.created_v1"
	EventReactionDeleted  = "im.message.reaction.deleted_v1"
	EventCardAction       = "card.action.trigger"
	EventURLVerification  = "url_verification"
)

// HTTP timeouts.
const (
	defaultAPITimeout = 10
)
