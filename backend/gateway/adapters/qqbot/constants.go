// Package qqbot implements a QQ Bot channel adapter for the SingerOS message gateway.
//
// The adapter connects to QQ Bot API v2 via WebSocket for inbound events and
// uses REST API for outbound message delivery. It supports:
//   - C2C (private chat) messages
//   - Group @-messages
//   - Guild channel messages
//   - Guild DM messages
//   - Interaction (button callback) events
//   - Media upload (images, files)
//   - Voice/audio STT
//
// Architecture:
//
//	REST (get token) → WebSocket (receive events) → normalize → gateway callback
//	Gateway response → normalize → REST (send messages)
//
// References:
//   - https://bot.qq.com/wiki/develop/api-v2/
//   - Hermes-Agent qqbot adapter (Python reference)
package qqbot

// OpCode values for the QQ Bot WebSocket gateway protocol.
const (
	OpDispatch       = 0  // server → client: event dispatch
	OpHeartbeat      = 1  // client ↔ server: heartbeat
	OpIdentify       = 2  // client → server: authenticate
	OpResume         = 6  // client → server: resume session
	OpReconnect      = 7  // server → client: request reconnect
	OpInvalidSession = 9  // server → client: session invalid
	OpHello          = 10 // server → client: connection established
	OpHeartbeatACK   = 11 // server → client: heartbeat acknowledged
)

// Event types received via WebSocket Dispatch (OpCode 0, field "t").
const (
	EventC2CMessageCreate       = "C2C_MESSAGE_CREATE"
	EventGroupAtMessageCreate   = "GROUP_AT_MESSAGE_CREATE"
	EventGuildAtMessageCreate   = "AT_MESSAGE_CREATE"
	EventDirectMessageCreate    = "DIRECT_MESSAGE_CREATE"
	EventGuildMessageCreate     = "MESSAGE_CREATE"
	EventInteractionCreate      = "INTERACTION_CREATE"
	EventFriendAdd              = "FRIEND_ADD"
	EventFriendDel              = "FRIEND_DEL"
	EventGroupAddRobot          = "GROUP_ADD_ROBOT"
	EventGroupDelRobot          = "GROUP_DEL_ROBOT"
	EventReady                  = "READY"
	EventResumed                = "RESUMED"
)

// Intent bitmask values. Each intent subscribes to a category of events.
const (
	IntentGuilds              = 1 << 0
	IntentGuildMembers        = 1 << 1
	IntentGuildMessages       = 1 << 9
	IntentGuildMsgReactions   = 1 << 10
	IntentDirectMessage       = 1 << 12
	IntentGroupAndC2CEvent    = 1 << 25
	IntentInteraction         = 1 << 26
	IntentMessageAudit        = 1 << 27
	IntentForumsEvent         = 1 << 28
	IntentAudioAction         = 1 << 29
	IntentPublicGuildMessages = 1 << 30 // 公域机器人 AT_MESSAGE_CREATE
)

// DefaultIntents is the default subscription set: C2C private chat, group @-message,
// guild channel @-message, guild DM, and interaction (button) events.
const DefaultIntents = IntentGroupAndC2CEvent | IntentPublicGuildMessages | IntentDirectMessage | IntentInteraction

// Message types for outbound REST API (msg_type field).
const (
	MsgTypeText     = 0 // text content
	MsgTypeImage    = 1 // (deprecated) image message
	MsgTypeMarkdown = 2 // markdown content
	MsgTypeArk      = 3 // ark template
	MsgTypeEmbed    = 4 // embed message
	MsgTypeMedia    = 7 // rich media (image/file via file_info)
)

// Chat types used internally for routing.
const (
	ChatTypeC2C   = "c2c"   // private chat with a user
	ChatTypeGroup = "group" // group @-message
	ChatTypeGuild = "guild" // guild channel message
	ChatTypeDM    = "dm"    // guild direct message
)

// API base URLs.
const (
	APIBaseURL   = "https://api.sgroup.qq.com"
	TokenBaseURL = "https://bots.qq.com"
)

// Media types for upload.
const (
	MediaTypeImage = 1
	MediaTypeVideo = 2
	MediaTypeVoice = 3
	MediaTypeFile  = 4
)

// WebSocket close codes and their handling strategies.
const (
	CloseCodeInvalidToken  = 4004 // token expired → refresh and reconnect
	CloseCodeRateLimited   = 4008 // rate limited → wait and reconnect
	CloseCodeConnTimeout   = 4009 // connection timeout → resumable
)

// isCloseCodeResumable returns true if the close code allows session resume.
func isCloseCodeResumable(code int) bool {
	return code == CloseCodeConnTimeout
}

// isCloseCodeFatal returns true if the close code should stop reconnection.
func isCloseCodeFatal(code int) bool {
	switch code {
	case 4001, 4002, 4003, 4005, 4010, 4011, 4012, 4013, 4014:
		return true
	}
	return false
}

// Reconnect backoff intervals (seconds).
var reconnectBackoff = []int{2, 5, 10, 30, 60}

// MaxMessageLength is the maximum outbound message length in characters.
const MaxMessageLength = 4000
