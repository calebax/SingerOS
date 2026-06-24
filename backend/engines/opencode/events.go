package opencode

import (
	"encoding/json"

	"github.com/insmtx/Leros/backend/internal/runtime/events"
	"github.com/ygpkg/yg-go/logs"
)

// ============================================================================
// SSE 消息事件解析
// ============================================================================

// handleSSEEvent 解析 SSE 事件并将消息相关事件转换为引擎事件。
// 消息事件包括：文本增量、工具调用、推理内容等。
func (st *runState) handleSSEEvent(event sseEvent) {
	logs.Debugf("[opencode] SSE event: type=%s id=%s props=%+v", event.Type, event.ID, event.Properties)

	st.mu.Lock()
	defer st.mu.Unlock()

	propsJSON, err := json.Marshal(event.Properties)
	if err != nil {
		return
	}

	switch event.Type {
	case "session.next.text.delta":
		var props textDeltaProps
		if err := json.Unmarshal(propsJSON, &props); err != nil {
			return
		}
		if props.Delta != "" {
			msgID := props.AssistantMessageID
			if msgID == "" {
				msgID = st.messageID
			}
			emitMessageDelta(st.evtChan, msgID, props.Delta)
		}

	case "session.next.text.started":
		// 仅记录 textID，不产生事件

	case "session.next.text.ended":
		var props textEndedProps
		if err := json.Unmarshal(propsJSON, &props); err != nil {
			return
		}
		st.lastTextEnded = props.Text

	case "session.next.tool.called":
		var props toolCalledProps
		if err := json.Unmarshal(propsJSON, &props); err != nil {
			return
		}
		sendEventPayloadTo(st.evtChan, events.EventToolCallStarted, events.ToolCallPayload{
			ToolCallID: props.CallID,
			Name:       props.Tool,
			Arguments:  props.Input,
		})

	case "session.next.tool.success":
		var props toolSuccessProps
		if err := json.Unmarshal(propsJSON, &props); err != nil {
			return
		}
		sendEventPayloadTo(st.evtChan, events.EventToolCallCompleted, events.ToolCallResultPayload{
			ToolCallID: props.CallID,
			Result:     props.Result,
		})

	case "session.next.tool.failed":
		var props toolFailedProps
		if err := json.Unmarshal(propsJSON, &props); err != nil {
			return
		}
		sendEventPayloadTo(st.evtChan, events.EventToolCallFailed, events.ToolCallResultPayload{
			ToolCallID: props.CallID,
			Error:      props.Error.Message,
			IsError:    true,
		})

	case "session.next.reasoning.delta":
		var props reasoningDeltaProps
		if err := json.Unmarshal(propsJSON, &props); err != nil {
			return
		}
		if props.Delta != "" {
			msgID := props.AssistantMessageID
			if msgID == "" {
				msgID = st.messageID
			}
			evt := events.NewReasoningDelta(msgID, props.Delta)
			sendEventDirect(st.evtChan, evt)
		}

	case "session.next.step.ended":
		var props stepEndedProps
		if err := json.Unmarshal(propsJSON, &props); err != nil {
			return
		}
		st.tokenUsage = &events.UsagePayload{
			InputTokens:  props.Tokens.Input,
			OutputTokens: props.Tokens.Output,
			TotalTokens:  props.Tokens.Input + props.Tokens.Output,
		}

	case "session.next.shell.started":
		// 以 message delta 展示
		emitMessageDelta(st.evtChan, st.messageID, "[shell] 正在执行命令...")

	case "session.next.agent.switched":
		// 记录但不产生事件
		logs.Infof("OpenCode agent switched: %s", string(propsJSON))

	case "session.next.model.switched":
		// 记录但不产生事件
		logs.Infof("OpenCode model switched: %s", string(propsJSON))

	case "server.connected":
		logs.Infof("OpenCode SSE connected")

	case "server.heartbeat":
		// 忽略心跳

	default:
	}
}
