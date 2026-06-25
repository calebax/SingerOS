package opencode

import (
	"testing"

	"github.com/insmtx/Leros/backend/internal/runtime/events"
)

func TestHandleSSEEventQuestionAskedEmitsQuestionEvent(t *testing.T) {
	st := &runState{evtChan: make(chan events.Event, 4)}

	st.handleSSEEvent(sseEvent{
		Type: "question.asked",
		Properties: map[string]any{
			"id":        "que_123",
			"sessionID": "ses_123",
			"tool": map[string]any{
				"callID":    "call_question",
				"messageID": "msg_question",
			},
			"questions": []any{
				map[string]any{
					"question": "今天是星期几？",
					"header":   "测试",
					"options": []any{
						map[string]any{"label": "星期四", "description": ""},
					},
				},
			},
		},
	})

	event := readEvent(t, st.evtChan)
	if event.Type != events.EventQuestionAsked {
		t.Fatalf("event type = %s, want %s", event.Type, events.EventQuestionAsked)
	}
	if event.Content != "今天是星期几？" {
		t.Fatalf("event content = %q", event.Content)
	}
	payload, err := events.DecodePayload[events.QuestionRequestPayload](&event)
	if err != nil {
		t.Fatalf("decode question payload: %v", err)
	}
	if payload.RequestID != "que_123" || payload.SessionID != "ses_123" {
		t.Fatalf("unexpected question identity: %#v", payload)
	}
	if payload.ToolCallID != "call_question" || payload.MessageID != "msg_question" {
		t.Fatalf("unexpected tool identity: %#v", payload)
	}
	if len(payload.Questions) != 1 || payload.Questions[0].Question != "今天是星期几？" {
		t.Fatalf("unexpected questions: %#v", payload.Questions)
	}
}

func TestHandleSSEEventFiltersConfiguredToolCall(t *testing.T) {
	st := &runState{evtChan: make(chan events.Event, 4)}

	st.handleSSEEvent(sseEvent{
		Type: "session.next.tool.called",
		Properties: map[string]any{
			"callID": "call_question",
			"tool":   "question",
			"input": map[string]any{
				"questions": []any{
					map[string]any{"question": "今天是星期几？"},
				},
			},
		},
	})
	st.handleSSEEvent(sseEvent{
		Type: "session.next.tool.success",
		Properties: map[string]any{
			"callID": "call_question",
			"result": map[string]any{
				"answers": []any{"星期四"},
			},
		},
	})

	select {
	case event := <-st.evtChan:
		t.Fatalf("unexpected event for question tool call lifecycle: %#v", event)
	default:
	}
}

func TestHandleSSEEventForwardsUnfilteredToolCall(t *testing.T) {
	st := &runState{evtChan: make(chan events.Event, 4)}

	st.handleSSEEvent(sseEvent{
		Type: "session.next.tool.called",
		Properties: map[string]any{
			"callID": "call_shell",
			"tool":   "shell",
			"input":  map[string]any{"command": "date"},
		},
	})

	event := readEvent(t, st.evtChan)
	if event.Type != events.EventToolCallStarted {
		t.Fatalf("event type = %s, want %s", event.Type, events.EventToolCallStarted)
	}
	payload, err := events.DecodePayload[events.ToolCallPayload](&event)
	if err != nil {
		t.Fatalf("decode tool payload: %v", err)
	}
	if payload.ToolCallID != "call_shell" || payload.Name != "shell" {
		t.Fatalf("unexpected tool payload: %#v", payload)
	}
}

func readEvent(t *testing.T, ch <-chan events.Event) events.Event {
	t.Helper()
	select {
	case event := <-ch:
		return event
	default:
		t.Fatal("expected event")
		return events.Event{}
	}
}
