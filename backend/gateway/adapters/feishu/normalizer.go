package feishu

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

// normalizeMessage converts a Feishu MessageEvent to a normalized MessageEnvelope.
func (a *Adapter) normalizeMessage(evt *MessageEvent) *types.MessageEnvelope {
	content := evt.Message
	text, attachments, msgType := a.parseContent(&content)

	// Convert open_id to user identifier
	userID := evt.Sender.SenderID.OpenID
	if userID == "" {
		userID = evt.Sender.SenderID.UserID
	}

	chatType := types.ChatTypeDM
	if content.ChatType == ChatTypeGroup {
		chatType = types.ChatTypeGroup
	} else if content.ChatType == ChatTypeP2P {
		chatType = types.ChatTypeDM
	}

	// Build mentions
	var mentions []types.Mention
	for _, m := range content.Mentions {
		var mentionID string
		if m.ID.OpenID != "" {
			mentionID = m.ID.OpenID
		} else if m.ID.UserID != "" {
			mentionID = m.ID.UserID
		}
		mentions = append(mentions, types.Mention{
			UserID:   mentionID,
			Username: m.Name,
		})
	}

	return &types.MessageEnvelope{
		MessageID:   content.MessageID,
		TraceID:     uuid.New().String(),
		Channel:     "feishu",
		MessageType: msgType,
		SessionKey: types.SessionKey{
			Channel:  "feishu",
			ChatType: chatType,
			ChatID:   content.ChatID,
			UserID:   userID,
			ThreadID: content.RootID,
		},
		Sender: types.SenderInfo{
			UserID: userID,
			IsBot:  evt.Sender.SenderType == "app",
		},
		Content: types.MessageContent{
			Text: text,
			Extra: map[string]any{
				"feishu_chat_type": content.ChatType,
				"feishu_msg_type":  content.MsgType,
			},
		},
		ReplyTo:     stringPtr(content.ParentID),
		Mentions:    mentions,
		Attachments: attachments,
		TransportMeta: &types.TransportMeta{
			PlatformMsgID: content.MessageID,
			ThreadRootID:  content.RootID,
		},
		CapabilitiesHint: &types.CapabilitiesHint{
			SupportsMarkdown: false,
			MaxMessageLen:    20000,
			SupportsMedia:    []string{"image", "file", "audio"},
		},
		RawEvent:   mustMarshal(evt),
		ReceivedAt: time.Now(),
	}
}

// parseContent extracts text and attachments from a Feishu MessageContent.
func (a *Adapter) parseContent(content *MessageContent) (text string, attachments []types.Attachment, msgType types.MessageType) {
	switch content.MsgType {
	case MsgTypeText:
		var tc TextContent
		if err := json.Unmarshal([]byte(content.Content), &tc); err == nil {
			text = tc.Text
		}
		return text, nil, types.MessageTypeText

	case MsgTypePost:
		var pc PostContent
		if err := json.Unmarshal([]byte(content.Content), &pc); err == nil {
			text = renderPostContent(&pc)
		}
		return text, nil, types.MessageTypeText

	case MsgTypeImage:
		var ic ImageContent
		if err := json.Unmarshal([]byte(content.Content), &ic); err == nil {
			attachments = append(attachments, types.Attachment{
				PlatformID: ic.ImageKey,
				MimeType:   "image/*",
				Width:      ic.Width,
				Height:     ic.Height,
			})
		}
		return "", attachments, types.MessageTypeImage

	case MsgTypeFile, MsgTypeAudio, MsgTypeMedia:
		var fc FileContent
		if err := json.Unmarshal([]byte(content.Content), &fc); err == nil {
			att := types.Attachment{
				PlatformID: fc.FileKey,
				Size:       int64(fc.Duration),
			}
			if fc.FileName != "" {
				att.MimeType = fc.FileName
			}
			attachments = append(attachments, att)
		}
		mt := types.MessageTypeFile
		if content.MsgType == MsgTypeAudio {
			mt = types.MessageTypeAudio
		}
		return "", attachments, mt

	case MsgTypeInteractive:
		var ic InteractiveContent
		if err := json.Unmarshal([]byte(content.Content), &ic); err == nil {
			var lines []string
			if ic.Header.Title.Content != "" {
				lines = append(lines, ic.Header.Title.Content)
			}
			for _, el := range ic.Elements {
				if el.Text.Content != "" {
					lines = append(lines, el.Text.Content)
				}
				for _, act := range el.Actions {
					if act.Text.Content != "" {
						lines = append(lines, fmt.Sprintf("[%s]", act.Text.Content))
					}
				}
			}
			text = strings.Join(lines, "\n")
		}
		return text, nil, types.MessageTypeEvent

	case MsgTypeShareChat:
		return "[shared chat]", nil, types.MessageTypeEvent

	case MsgTypeMergeForward:
		return "[merged messages]", nil, types.MessageTypeEvent

	default:
		return fmt.Sprintf("[%s message]", content.MsgType), nil, types.MessageTypeEvent
	}
}

// renderPostContent converts a Feishu post message to plain text.
func renderPostContent(pc *PostContent) string {
	var lines []string
	if pc.Title != "" {
		lines = append(lines, pc.Title+"\n")
	}
	for _, row := range pc.Content {
		var rowTexts []string
		for _, el := range row {
			switch el.Tag {
			case "text":
				if el.Text != "" {
					rowTexts = append(rowTexts, el.Text)
				}
			case "at":
				if el.UserName != "" {
					rowTexts = append(rowTexts, "@"+el.UserName)
				}
			case "a":
				if el.Text != "" {
					rowTexts = append(rowTexts, el.Text)
				}
			case "img":
				rowTexts = append(rowTexts, "[image]")
			case "file":
				rowTexts = append(rowTexts, "[file]")
			}
		}
		if len(rowTexts) > 0 {
			lines = append(lines, strings.Join(rowTexts, ""))
		}
	}
	return strings.Join(lines, "\n")
}
