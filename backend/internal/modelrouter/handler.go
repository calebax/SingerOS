package modelrouter

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/pkg/llmprotocol"
)

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

		c.Request = c.Request.WithContext(injectBusinessIDs(c.Request.Context(), store, c))

		if isStream {
			handleStreamResponse(c, caller, orgID, modelCfg, upstreamBodyBytes, entryProtocol, upstreamProtocol, entryAdapter, upstreamAdapter, dl)
		} else {
			handleNonStreamResponse(c, caller, orgID, modelCfg, upstreamBodyBytes, entryProtocol, upstreamProtocol, entryAdapter, upstreamAdapter, dl)
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
// e.g., "/gemini-2.0-flash:generateContent" -> "gemini-2.0-flash"
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
