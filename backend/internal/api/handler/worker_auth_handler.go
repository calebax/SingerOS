package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/api/dto"
	"github.com/insmtx/Leros/backend/internal/api/middleware"
)

const headerWorkerBootstrapToken = "X-Worker-Bootstrap-Token"

// WorkerAuthHandler exchanges worker bootstrap tokens for short-lived
// access tokens by delegating to a TokenParser.
type WorkerAuthHandler struct {
	parser middleware.TokenParser
}

type issueWorkerTokenRequest struct {
	OrgID          uint   `json:"org_id" binding:"required"`
	WorkerID       uint   `json:"worker_id" binding:"required"`
	BootstrapToken string `json:"bootstrap_token,omitempty"`
}

type issueWorkerTokenResponse struct {
	AuthToken string `json:"auth_token"`
	ExpiredAt int64  `json:"expired_at"`
	TokenType string `json:"token_type"`
}

// NewWorkerAuthHandler creates a worker auth handler backed by the given
// TokenParser.
func NewWorkerAuthHandler(parser middleware.TokenParser) *WorkerAuthHandler {
	return &WorkerAuthHandler{parser: parser}
}

// RegisterWorkerAuthRoutes registers worker auth routes.
func RegisterWorkerAuthRoutes(r gin.IRouter, parser middleware.TokenParser) {
	h := NewWorkerAuthHandler(parser)
	r.POST("/workers/token", h.IssueToken)
}

// IssueToken exchanges a worker bootstrap token for a short-lived worker access token.
func (h *WorkerAuthHandler) IssueToken(ctx *gin.Context) {
	var req issueWorkerTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, err.Error()))
		return
	}

	bootstrapToken := strings.TrimSpace(ctx.GetHeader(headerWorkerBootstrapToken))
	if bootstrapToken == "" {
		bootstrapToken = strings.TrimSpace(req.BootstrapToken)
	}
	if bootstrapToken == "" {
		ctx.JSON(http.StatusUnauthorized, dto.Error(dto.CodeInternalError, "worker bootstrap token is required"))
		return
	}

	token, expiredAt, err := h.parser.IssueWorker(ctx.Request.Context(), req.OrgID, req.WorkerID, bootstrapToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, dto.Error(dto.CodeInternalError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, dto.Success(issueWorkerTokenResponse{
		AuthToken: token,
		ExpiredAt: expiredAt,
		TokenType: "Bearer",
	}))
}
