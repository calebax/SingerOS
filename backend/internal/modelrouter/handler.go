package modelrouter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/llm"
	"github.com/insmtx/Leros/backend/pkg/llmprotocol"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ModelStore — minimal in-handler model config resolution
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ModelStore holds UpstreamConfig entries keyed by model name.
// It is safe for concurrent use.
type ModelStore struct {
	configs map[string]*UpstreamConfig
	caller  llm.Caller
	orgID   uint
	mu      sync.RWMutex
}

// NewModelStore creates an isolated model routing store.
func NewModelStore() *ModelStore {
	return &ModelStore{configs: make(map[string]*UpstreamConfig)}
}

// SetCaller sets the llm.Caller used for upstream LLM calls.
func (s *ModelStore) SetCaller(caller llm.Caller) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.caller = caller
}

// SetOrgID sets the org ID used for call recording.
func (s *ModelStore) SetOrgID(orgID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orgID = orgID
}

// Put registers an upstream configuration for a model.
// If cfg.Protocol is not explicitly set and cfg.Provider is non-empty,
// the protocol is inferred from the provider.
func (s *ModelStore) Put(cfg UpstreamConfig) {
	if cfg.Protocol == "" && cfg.Provider != "" {
		cfg.Protocol = protocolForProvider(cfg.Provider)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.configs == nil {
		s.configs = make(map[string]*UpstreamConfig)
	}
	cp := cfg
	s.configs[cfg.ModelName] = &cp
}

// Resolve returns the UpstreamConfig for the given model name.
func (s *ModelStore) Resolve(model string) (*UpstreamConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[model]
	if !ok {
		return nil, fmt.Errorf("modelrouter: no upstream config for model %q", model)
	}
	cp := *cfg
	return &cp, nil
}

func (s *ModelStore) getCaller() llm.Caller {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.caller
}

func (s *ModelStore) getOrgID() uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orgID
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Route Registration
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// RegisterRoutes registers model routing endpoints on the given Gin router.
// Each endpoint supports all entry protocols and transparently converts
// between protocols when upstream targets a different protocol.
func RegisterRoutes(r gin.IRouter, store *ModelStore) {
	if store == nil {
		return
	}
	// NOTE: caller (worker/router) already mounts under /v1/ prefix.
	// Register directly on the given router to avoid double-wrapping.
	r.POST("/chat/completions", handleModelRoute(store, llmprotocol.ProtocolOpenAIChat))
	r.POST("/messages", handleModelRoute(store, llmprotocol.ProtocolAnthropicMessages))
	r.POST("/responses", handleModelRoute(store, llmprotocol.ProtocolOpenAIResponses))
	// Gemini: use wildcard because Gin cannot handle ":model:generateContent" in one segment
	r.POST("/models/*modelAction", handleModelRoute(store, llmprotocol.ProtocolGemini))
}

// handleModelRoute returns a Gin handler that routes model requests through protocol conversion.
// Upstream LLM calls are delegated to llm.Caller for unified accounting.
func handleModelRoute(store *ModelStore, entryProtocol llmprotocol.Protocol) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, newEntryError(entryProtocol, "failed to read request body"))
			return
		}

		caller := store.getCaller()
		if caller == nil {
			c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, "llm caller not configured"))
			return
		}

		debugEnabled := os.Getenv("LEROS_MODELROUTER_DEBUG") == "true"
		dl := NewDebugLogger(debugEnabled)
		defer dl.Close()

		dl.LogOriginalRequest(body)

		model := extractModelField(body)
		if model == "" && entryProtocol == llmprotocol.ProtocolGemini {
			model = extractGeminiModelFromPath(c.Param("modelAction"))
		}
		if model == "" {
			c.JSON(http.StatusBadRequest, newEntryError(entryProtocol, "model field is required"))
			return
		}

		cfg, err := store.Resolve(model)
		if err != nil {
			c.JSON(http.StatusBadRequest, newEntryError(entryProtocol, err.Error()))
			return
		}

		isStream := isStreamRequest(body)
		dl.LogRequestMeta(entryProtocol, cfg.Protocol, model, isStream)

		var raw map[string]interface{}
		if err := sonic.Unmarshal(body, &raw); err != nil {
			c.JSON(http.StatusBadRequest, newEntryError(entryProtocol, "invalid JSON request body"))
			return
		}

		entryAdapter, err := llmprotocol.GetAdapter(entryProtocol)
		if err != nil {
			c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, "entry protocol adapter not available"))
			return
		}

		ir, err := entryAdapter.DecodeRequest(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, newEntryError(entryProtocol, fmt.Sprintf("decode request: %v", err)))
			return
		}
		dl.LogIRDecoded(ir)

		upstreamProtocol := cfg.Protocol
		targetCaps := llmprotocol.CapabilitiesForProtocol(upstreamProtocol)
		normalizedIR, _, err := llmprotocol.NormalizeRequest(ir, targetCaps)
		if err != nil {
			c.JSON(http.StatusBadRequest, newEntryError(entryProtocol, fmt.Sprintf("request incompatible with target protocol: %v", err)))
			return
		}
		dl.LogIRNormalized(normalizedIR)

		normalizedIR.Model = cfg.ModelName

		upstreamAdapter, err := llmprotocol.GetAdapter(upstreamProtocol)
		if err != nil {
			c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, "upstream protocol adapter not available"))
			return
		}

		upstreamBody, err := upstreamAdapter.EncodeRequest(normalizedIR)
		if err != nil {
			c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, fmt.Sprintf("encode upstream request: %v", err)))
			return
		}

		upstreamBodyBytes, err := marshalJSON(upstreamBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, "marshal upstream body failed"))
			return
		}
		dl.LogUpstreamRequest(upstreamBodyBytes)

		orgID := store.getOrgID()
		modelCfg := cfg.ToModelConfig()

		c.Request = c.Request.WithContext(injectBusinessIDs(c.Request.Context(), c))

		if isStream {
			handleStreamResponse(c, caller, orgID, modelCfg, upstreamBodyBytes, entryProtocol, upstreamProtocol, entryAdapter, upstreamAdapter, dl)
		} else {
			handleNonStreamResponse(c, caller, orgID, modelCfg, upstreamBodyBytes, entryProtocol, upstreamProtocol, entryAdapter, upstreamAdapter, dl)
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Non-stream response handling
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func handleNonStreamResponse(
	c *gin.Context,
	caller llm.Caller,
	orgID uint,
	modelCfg *llm.ModelConfig,
	upstreamBodyBytes []byte,
	entryProtocol, upstreamProtocol llmprotocol.Protocol,
	entryAdapter, upstreamAdapter llmprotocol.ProtocolAdapter,
	dl *DebugLogger,
) {
	result, err := caller.CallRaw(c.Request.Context(), orgID, modelCfg, upstreamBodyBytes)
	if err != nil {
		dl.LogError("call_raw", err)
		var upErr *llm.UpstreamError
		if errors.As(err, &upErr) {
			statusCode := upErr.StatusCode
			if statusCode >= 500 {
				statusCode = http.StatusBadGateway
			}
			c.JSON(statusCode, parseUpstreamErrorBody(upErr.Body, entryProtocol))
		} else if result != nil && len(result.RawResponseBody) > 0 {
			c.JSON(http.StatusBadGateway, parseUpstreamErrorBody(result.RawResponseBody, entryProtocol))
		} else {
			handleCallError(c, entryProtocol, err)
		}
		return
	}

	respBody := result.RawResponseBody
	dl.LogUpstreamResponse(respBody)

	var rawResp map[string]interface{}
	if err := sonic.Unmarshal(respBody, &rawResp); err != nil {
		c.JSON(http.StatusBadGateway, newEntryError(entryProtocol, "invalid upstream response"))
		return
	}

	irResp, err := upstreamAdapter.DecodeResponse(rawResp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, fmt.Sprintf("decode upstream response: %v", err)))
		return
	}

	entryBody, err := entryAdapter.EncodeResponse(irResp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, fmt.Sprintf("encode entry response: %v", err)))
		return
	}

	entryBytes, err := marshalJSON(entryBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, newEntryError(entryProtocol, "marshal entry response failed"))
		return
	}

	dl.LogEntryResponse(entryBytes)
	c.Data(http.StatusOK, "application/json", entryBytes)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Stream response handling
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func handleStreamResponse(
	c *gin.Context,
	caller llm.Caller,
	orgID uint,
	modelCfg *llm.ModelConfig,
	upstreamBodyBytes []byte,
	entryProtocol, upstreamProtocol llmprotocol.Protocol,
	entryAdapter, upstreamAdapter llmprotocol.ProtocolAdapter,
	dl *DebugLogger,
) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	c.Writer.WriteHeaderNow()
	c.Writer.Flush()

	var once sync.Once
	closeDone := make(chan struct{})
	w := c.Writer

	sink := &rawSSESink{
		writer:           w,
		flusher:          w,
		entryProtocol:    entryProtocol,
		upstreamProtocol: upstreamProtocol,
		upstreamAdapter:  upstreamAdapter,
		entryAdapter:     entryAdapter,
		dl:               dl,
		aggregator:       llmprotocol.NewStreamAggregator(),
		closeOnce:        &once,
		closeDone:        closeDone,
	}

	result, err := caller.StreamRaw(c.Request.Context(), orgID, modelCfg, upstreamBodyBytes, sink)
	if err != nil {
		dl.LogError("stream_raw", err)
		if result != nil && len(result.RawResponseBody) > 0 {
			sink.flushError(errors.New(string(result.RawResponseBody)))
		} else {
			sink.flushError(err)
		}
		return
	}

	sink.finalize()
}

// rawSSESink implements llm.RawChunkSink to receive raw SSE chunks from CallerHTTP
// and perform protocol conversion + SSE formatting.
type rawSSESink struct {
	writer           http.ResponseWriter
	flusher          http.Flusher
	entryProtocol    llmprotocol.Protocol
	upstreamProtocol llmprotocol.Protocol
	upstreamAdapter  llmprotocol.ProtocolAdapter
	entryAdapter     llmprotocol.ProtocolAdapter
	dl               *DebugLogger
	aggregator       *llmprotocol.StreamAggregator
	state            sinkState
	closeOnce        *sync.Once
	closeDone        chan struct{}
	mu               sync.Mutex
}

type sinkState struct {
	upstream    interface{}
	entry       interface{}
	eventType   string
	currentData strings.Builder
}

func (s *rawSSESink) initState() {
	if s.state.upstream == nil {
		s.state.upstream = s.upstreamAdapter.NewStreamState()
	}
	if s.state.entry == nil {
		s.state.entry = s.entryAdapter.NewStreamState()
	}
}

func (s *rawSSESink) EmitRawChunk(ctx context.Context, chunk []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.initState()

	s.dl.LogUpstreamStreamChunk(chunk)

	if s.entryProtocol == s.upstreamProtocol {
		if _, err := s.writer.Write(chunk); err != nil {
			return err
		}
		s.flusher.Flush()
		s.dl.LogEntryStreamChunk(chunk)
		return nil
	}

	text := string(chunk)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			s.state.eventType = strings.TrimPrefix(line, "event: ")
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				s.dl.LogUpstreamStreamChunk([]byte("data: [DONE]\n\n"))
				s.flushCurrentData()
				s.writeIREvents(s.aggregator.Finalize())
				if s.upstreamProtocol == llmprotocol.ProtocolOpenAIChat {
					s.dl.LogEntryStreamChunk([]byte("data: [DONE]\n\n"))
					s.writer.Write([]byte("data: [DONE]\n\n"))
					s.flusher.Flush()
				}
				return nil
			}
			s.state.currentData.WriteString(data)
			continue
		}
		if line == "" && s.state.currentData.Len() > 0 {
			s.flushCurrentData()
		}
	}

	return nil
}

func (s *rawSSESink) flushCurrentData() {
	if s.state.currentData.Len() == 0 {
		return
	}
	dataStr := s.state.currentData.String()
	s.state.currentData.Reset()

	var rawUpstream map[string]interface{}
	if err := sonic.Unmarshal([]byte(dataStr), &rawUpstream); err != nil {
		return
	}

	irEvents, err := s.upstreamAdapter.DecodeStreamEvent(rawUpstream, s.state.upstream)
	if err != nil {
		return
	}

	for _, irEvt := range irEvents {
		fixedEvents := s.aggregator.ProcessIREvent(irEvt)
		s.writeIREvents(fixedEvents)
	}

	s.state.eventType = ""
}

func (s *rawSSESink) writeIREvents(events []*llmprotocol.IRStreamEvent) {
	for _, evt := range events {
		payloads, err := s.entryAdapter.EncodeStreamEvent(evt, s.state.entry)
		if err != nil {
			continue
		}
		for _, payload := range payloads {
			payloadBytes, err := marshalJSON(payload)
			if err != nil {
				continue
			}
			evtType := s.state.eventType
			if evtType == "" {
				if v, ok := payload["type"].(string); ok {
					evtType = v
				}
			}
			formatted := formatSSE(s.entryProtocol, evtType, payloadBytes)
			s.dl.LogEntryStreamChunk(formatted)
			if _, err := s.writer.Write(formatted); err != nil {
				return
			}
			s.flusher.Flush()
		}

		if evt.Type == llmprotocol.IRStreamDone && s.entryProtocol == llmprotocol.ProtocolOpenAIChat {
			s.dl.LogEntryStreamChunk([]byte("data: [DONE]\n\n"))
			s.writer.Write([]byte("data: [DONE]\n\n"))
			s.flusher.Flush()
		}
	}
}

func (s *rawSSESink) finalize() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.aggregator != nil && !s.aggregator.IsDone() {
		s.writeIREvents(s.aggregator.Finalize())
	}

	s.closeOnce.Do(func() {
		close(s.closeDone)
	})
}

func (s *rawSSESink) flushError(err error) {
	errBytes, _ := marshalJSON(newEntryError(s.entryProtocol, err.Error()))
	formatted := formatSSE(s.entryProtocol, "error", errBytes)
	s.writer.Write(formatted)
	s.flusher.Flush()
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SSE formatting
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func formatSSE(proto llmprotocol.Protocol, eventType string, data []byte) []byte {
	switch proto {
	case llmprotocol.ProtocolOpenAIChat:
		return []byte(fmt.Sprintf("data: %s\n\n", string(data)))
	default: // Anthropic, Responses, Gemini use event: header
		return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(data)))
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Error handling
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func handleCallError(c *gin.Context, entryProtocol llmprotocol.Protocol, err error) {
	c.JSON(http.StatusBadGateway, newEntryError(entryProtocol, err.Error()))
}

// newEntryError creates an entry protocol error response.
func newEntryError(proto llmprotocol.Protocol, message string) interface{} {
	return encodeErrorForProtocol(message, "invalid_request_error", proto)
}

// parseUpstreamErrorBody parses the raw upstream error body and encodes it for the entry protocol.
func parseUpstreamErrorBody(body []byte, entryProtocol llmprotocol.Protocol) interface{} {
	if len(body) == 0 {
		return newEntryError(entryProtocol, "upstream returned an error")
	}

	var raw map[string]interface{}
	if err := sonic.Unmarshal(body, &raw); err != nil {
		return newEntryError(entryProtocol, fmt.Sprintf("upstream error: %s", string(body)))
	}

	message := ""
	errType := "upstream_error"

	if getString(raw, "type") == "error" {
		if errObj, ok := raw["error"].(map[string]interface{}); ok {
			message = getString(errObj, "message")
			errType = getString(errObj, "type")
		}
	} else if errObj, ok := raw["error"].(map[string]interface{}); ok {
		message = getString(errObj, "message")
		errType = getString(errObj, "type")
	} else if msg := getString(raw, "message"); msg != "" {
		message = msg
	}

	if message == "" {
		message = string(body)
	}
	if errType == "" {
		errType = "upstream_error"
	}

	return encodeErrorForProtocol(message, errType, entryProtocol)
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// encodeErrorForProtocol encodes an error message and type into the entry protocol's error format.
func encodeErrorForProtocol(message, errType string, proto llmprotocol.Protocol) interface{} {
	switch proto {
	case llmprotocol.ProtocolAnthropicMessages:
		return map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    errType,
				"message": message,
			},
		}
	default:
		return map[string]interface{}{
			"error": map[string]interface{}{
				"message": message,
				"type":    errType,
			},
		}
	}
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// Request parsing helpers
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func extractModelField(body []byte) string {
	var raw struct {
		Model string `json:"model"`
	}
	if err := sonic.Unmarshal(body, &raw); err != nil {
		return ""
	}
	return strings.TrimSpace(raw.Model)
}

// extractGeminiModelFromPath extracts the model name from a Gemini URL action parameter.
// e.g., "/gemini-2.0-flash:generateContent" → "gemini-2.0-flash"
func extractGeminiModelFromPath(action string) string {
	action = strings.TrimPrefix(action, "/")
	colonIdx := strings.LastIndex(action, ":")
	if colonIdx < 0 {
		return ""
	}
	return action[:colonIdx]
}

func isStreamRequest(body []byte) bool {
	var raw struct {
		Stream bool `json:"stream"`
	}
	if err := sonic.Unmarshal(body, &raw); err != nil {
		return false
	}
	return raw.Stream
}

func marshalJSON(v interface{}) ([]byte, error) {
	return sonic.ConfigStd.Marshal(v)
}

// injectBusinessIDs 从 HTTP 请求头中提取业务 ID，注入到 context 中。
// 客户端通过 X-Leros-Project-Id / X-Leros-Session-Id / X-Leros-Message-Id /
// X-Leros-Assistant-Id / X-Leros-Uin 请求头传递业务 ID。
func injectBusinessIDs(ctx context.Context, c *gin.Context) context.Context {
	if v := parseHeaderUint(c, "X-Leros-Project-Id"); v > 0 {
		ctx = llm.WithCtxUint(ctx, llm.CtxProjectID, v)
	}
	if v := parseHeaderUint(c, "X-Leros-Session-Id"); v > 0 {
		ctx = llm.WithCtxUint(ctx, llm.CtxSessionID, v)
	}
	if v := parseHeaderUint(c, "X-Leros-Message-Id"); v > 0 {
		ctx = llm.WithCtxUint(ctx, llm.CtxMessageID, v)
	}
	if v := parseHeaderUint(c, "X-Leros-Assistant-Id"); v > 0 {
		ctx = llm.WithCtxUint(ctx, llm.CtxAssistantID, v)
	}
	if v := parseHeaderUint(c, "X-Leros-Uin"); v > 0 {
		ctx = llm.WithCtxUint(ctx, llm.CtxUin, v)
	}
	return ctx
}

func parseHeaderUint(c *gin.Context, key string) uint {
	val := strings.TrimSpace(c.GetHeader(key))
	if val == "" {
		return 0
	}
	n, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0
	}
	return uint(n)
}
