package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/infra/filestore"
)

const presignTokenQuery = "token"
const presignExpiresQuery = "expires"

// RegisterPresignedRoutes registers routes that handle presigned URL consumption
func RegisterPresignedRoutes(r gin.IRouter) {
	r.PUT("/:bucket/*key", handlePresignedPut)
	r.GET("/:bucket/*key", handlePresignedGet)
}

func handlePresignedPut(ctx *gin.Context) {
	token := strings.TrimSpace(ctx.Query(presignTokenQuery))
	expires := strings.TrimSpace(ctx.Query(presignExpiresQuery))
	if token == "" || expires == "" {
		ctx.String(http.StatusBadRequest, "missing token or expires query parameter")
		return
	}

	bucket := strings.TrimSpace(ctx.Param("bucket"))
	key := strings.TrimPrefix(ctx.Param("key"), "/")

	if bucket == "" || key == "" {
		ctx.String(http.StatusBadRequest, "bucket and key are required")
		return
	}

	if err := filestore.VerifyPresignedToken(
		filestore.SignSecret(), bucket, key, "put", token, expires,
	); err != nil {
		if errors.Is(err, filestore.ErrPresignExpired) {
			ctx.String(http.StatusForbidden, "presigned url expired")
			return
		}
		if errors.Is(err, filestore.ErrPresignOpMismatch) {
			ctx.String(http.StatusForbidden, "operation mismatch")
			return
		}
		if errors.Is(err, filestore.ErrPresignKeyMismatch) {
			ctx.String(http.StatusForbidden, "key mismatch")
			return
		}
		ctx.String(http.StatusForbidden, "invalid presigned token")
		return
	}

	contentType := ctx.GetHeader("Content-Type")
	if err := filestore.HandlePresignedPut(ctx.Request.Context(), bucket, key, ctx.Request.Body, contentType); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload failed: %v", err))
		return
	}

	ctx.String(http.StatusOK, "uploaded")
}

func handlePresignedGet(ctx *gin.Context) {
	token := strings.TrimSpace(ctx.Query(presignTokenQuery))
	expires := strings.TrimSpace(ctx.Query(presignExpiresQuery))
	if token == "" || expires == "" {
		ctx.String(http.StatusBadRequest, "missing token or expires query parameter")
		return
	}

	bucket := strings.TrimSpace(ctx.Param("bucket"))
	key := strings.TrimPrefix(ctx.Param("key"), "/")

	if bucket == "" || key == "" {
		ctx.String(http.StatusBadRequest, "bucket and key are required")
		return
	}

	if err := filestore.VerifyPresignedToken(
		filestore.SignSecret(), bucket, key, "get", token, expires,
	); err != nil {
		if errors.Is(err, filestore.ErrPresignExpired) {
			ctx.String(http.StatusForbidden, "presigned url expired")
			return
		}
		if errors.Is(err, filestore.ErrPresignOpMismatch) {
			ctx.String(http.StatusForbidden, "operation mismatch")
			return
		}
		if errors.Is(err, filestore.ErrPresignKeyMismatch) {
			ctx.String(http.StatusForbidden, "key mismatch")
			return
		}
		ctx.String(http.StatusForbidden, "invalid presigned token")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			ctx.String(http.StatusInternalServerError, "internal error")
		}
	}()
	body, info, err := filestore.HandlePresignedGet(ctx.Request.Context(), bucket, key)
	if err != nil {
		ctx.String(http.StatusNotFound, "object not found")
		return
	}
	defer body.Close()

	if info.ContentType != "" {
		ctx.Header("Content-Type", info.ContentType)
	}
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, key))
	if info.Size > 0 {
		ctx.Header("Content-Length", fmt.Sprintf("%d", info.Size))
	}
	ctx.Status(http.StatusOK)
	if _, err := io.Copy(ctx.Writer, body); err != nil {
		ctx.Error(err)
	}
}
