package qqbot

import (
	"time"

	"github.com/google/uuid"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// normalizeC2C converts a C2C private chat event to a MessageEnvelope.
func (a *Adapter) normalizeC2C(evt *C2CMessageEvent) *types.MessageEnvelope {
	if evt.Content == "" {
		return nil
	}

	return &types.MessageEnvelope{
		MessageID:   evt.ID,
		TraceID:     uuid.New().String(),
		Channel:     "qqbot",
		MessageType: types.MessageTypeText,
		SessionKey: types.SessionKey{
			Channel:  "qqbot",
			ChatType: types.ChatTypeDM,
			ChatID:   evt.Author.UserOpenID,
			UserID:   evt.Author.UserOpenID,
		},
		Sender: types.SenderInfo{
			UserID:   evt.Author.UserOpenID,
		},
		Content: types.MessageContent{
			Text: a.extractText(evt.Content),
			Extra: map[string]any{
				"qq_chat_type":  ChatTypeC2C,
				"qq_user_openid": evt.Author.UserOpenID,
			},
		},
		RawEvent:   mustMarshal(evt),
		ReceivedAt: time.Now(),
	}
}

// normalizeGroup converts a group @-message event to a MessageEnvelope.
func (a *Adapter) normalizeGroup(evt *GroupAtMessageEvent) *types.MessageEnvelope {
	if evt.Content == "" {
		return nil
	}

	return &types.MessageEnvelope{
		MessageID:   evt.ID,
		TraceID:     uuid.New().String(),
		Channel:     "qqbot",
		MessageType: types.MessageTypeText,
		SessionKey: types.SessionKey{
			Channel:  "qqbot",
			ChatType: types.ChatTypeGroup,
			ChatID:   evt.GroupOpenID,
			UserID:   evt.Author.MemberOpenID,
		},
		Sender: types.SenderInfo{
			UserID:   evt.Author.MemberOpenID,
		},
		Content: types.MessageContent{
			Text: a.extractText(evt.Content),
			Extra: map[string]any{
				"qq_chat_type":   ChatTypeGroup,
				"qq_group_openid": evt.GroupOpenID,
				"qq_member_openid": evt.Author.MemberOpenID,
			},
		},
		RawEvent:   mustMarshal(evt),
		ReceivedAt: time.Now(),
	}
}

// normalizeGuild converts a guild channel @-message event to a MessageEnvelope.
func (a *Adapter) normalizeGuild(evt *GuildAtMessageEvent) *types.MessageEnvelope {
	userID := evt.Author.ID
	if userID == "" {
		userID = evt.Author.Username
	}

	text := a.extractText(evt.Content)

	// Collect attachment URLs
	var mediaURLs []string
	for _, att := range evt.Attachments {
		if att.URL != "" {
			mediaURLs = append(mediaURLs, att.URL)
		}
	}

	msgType := types.MessageTypeText
	if len(mediaURLs) > 0 && text == "" {
		msgType = types.MessageTypeImage
	}

	return &types.MessageEnvelope{
		MessageID:   evt.ID,
		TraceID:     uuid.New().String(),
		Channel:     "qqbot",
		MessageType: msgType,
		SessionKey: types.SessionKey{
			Channel:  "qqbot",
			ChatType: types.ChatTypeChannel,
			ChatID:   evt.ChannelID,
			UserID:   userID,
		},
		Sender: types.SenderInfo{
			UserID:   userID,
			Username: evt.Author.Username,
			Avatar:   evt.Author.Avatar,
			IsBot:    evt.Author.Bot,
		},
		Content: types.MessageContent{
			Text:      text,
			MediaURLs: mediaURLs,
			Extra: map[string]any{
				"qq_chat_type":  ChatTypeGuild,
				"qq_guild_id":   evt.GuildID,
				"qq_channel_id": evt.ChannelID,
			},
		},
		RawEvent:   mustMarshal(evt),
		ReceivedAt: time.Now(),
	}
}

// normalizeDM converts a guild DM event to a MessageEnvelope.
func (a *Adapter) normalizeDM(evt *DirectMessageEvent) *types.MessageEnvelope {
	userID := evt.Author.ID
	if userID == "" {
		userID = evt.MemberOpenID
	}

	text := a.extractText(evt.Content)

	var mediaURLs []string
	for _, att := range evt.Attachments {
		if att.URL != "" {
			mediaURLs = append(mediaURLs, att.URL)
		}
	}

	msgType := types.MessageTypeText
	if len(mediaURLs) > 0 && text == "" {
		msgType = types.MessageTypeImage
	}

	return &types.MessageEnvelope{
		MessageID:   evt.ID,
		TraceID:     uuid.New().String(),
		Channel:     "qqbot",
		MessageType: msgType,
		SessionKey: types.SessionKey{
			Channel:  "qqbot",
			ChatType: types.ChatTypeDM,
			ChatID:   evt.GuildID, // QQ DM 用 guild_id
			UserID:   userID,
		},
		Sender: types.SenderInfo{
			UserID:   userID,
			Username: evt.Author.Username,
			Avatar:   evt.Author.Avatar,
			IsBot:    evt.Author.Bot,
		},
		Content: types.MessageContent{
			Text:      text,
			MediaURLs: mediaURLs,
			Extra: map[string]any{
				"qq_chat_type":    ChatTypeDM,
				"qq_guild_id":     evt.GuildID,
				"qq_member_openid": evt.MemberOpenID,
			},
		},
		RawEvent:   mustMarshal(evt),
		ReceivedAt: time.Now(),
	}
}

// normalizeInteraction converts a button interaction event to a MessageEnvelope.
func (a *Adapter) normalizeInteraction(evt *InteractionEvent) *types.MessageEnvelope {
	userID := ""
	if evt.Member != nil {
		userID = evt.Member.ID
	}
	if userID == "" && evt.User != nil {
		userID = evt.User.ID
	}

	chatType := types.ChatTypeDM
	chatID := evt.GroupOpenID
	if chatID == "" && evt.ChannelID != "" {
		chatType = types.ChatTypeChannel
		chatID = evt.ChannelID
	} else if chatID == "" && evt.GuildID != "" {
		chatType = types.ChatTypeDM
		chatID = evt.GuildID
	}

	return &types.MessageEnvelope{
		MessageID:   evt.ID,
		TraceID:     uuid.New().String(),
		Channel:     "qqbot",
		MessageType: types.MessageTypeEvent,
		SessionKey: types.SessionKey{
			Channel:  "qqbot",
			ChatType: chatType,
			ChatID:   chatID,
			UserID:   userID,
		},
		Sender: types.SenderInfo{
			UserID: userID,
		},
		Content: types.MessageContent{
			Text: evt.Data.ButtonData,
			Extra: map[string]any{
				"qq_chat_type":    "interaction",
				"qq_button_id":    evt.Data.ButtonID,
				"qq_button_data":  evt.Data.ButtonData,
				"qq_interaction_id": evt.ID,
			},
		},
		SkillHint: stringPtr("interaction"),
		RawEvent:  mustMarshal(evt),
		ReceivedAt: time.Now(),
	}
}

func stringPtr(s string) *string {
	return &s
}
