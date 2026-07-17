package middleware

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ygpkg/yg-go/apis/constants"
	"github.com/ygpkg/yg-go/logs"
	"gorm.io/gorm"

	localauth "github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/types"
)

const (
	headerKeyRequestID = "X-Request-ID"
	headerKeyTraceID   = "X-Trace-ID"
)

// CallerMiddleware .
func CallerMiddleware(jwtSecret string, database *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqID := ctx.Request.Header.Get(headerKeyRequestID)
		if reqID == "" {
			reqID = strings.ReplaceAll(uuid.NewString(), "-", "")
		}
		traceID := ctx.Request.Header.Get(headerKeyTraceID)
		if traceID == "" {
			traceID = reqID
		}
		ctx.Set(constants.CtxKeyRequestID, reqID)
		ctx.Header(headerKeyRequestID, reqID)
		logs.SetContextFields(ctx, "req_id", reqID)
		requestCtx := context.WithValue(ctx.Request.Context(), constants.CtxKeyRequestID, reqID)
		ctx.Request = ctx.Request.WithContext(logs.WithContextFields(
			requestCtx, "req_id", reqID,
		))

		caller := parseCallerFromRequest(ctx, jwtSecret, database)

		trace := &types.Trace{
			RequestID: reqID,
			TraceID:   traceID,
			SpanID:    []string{},
		}
		localauth.WithGinContext(ctx, caller, trace)
		ctx.Request = ctx.Request.WithContext(localauth.WithContext(ctx.Request.Context(), caller, trace))
		ctx.Next()
	}
}

func parseCallerFromRequest(ctx *gin.Context, jwtSecret string, database *gorm.DB) *types.Caller {
	if os.Getenv("LEROS_DEV") == "true" {
		return &types.Caller{
			Uin:   1,
			OrgID: 1,
			Kind:  types.CallerKindUser,
			State: types.AuthStateSucc,
		}
	}
	authHeader := ctx.Request.Header.Get("Authorization")
	if authHeader == "" {
		return &types.Caller{
			Uin:   0,
			OrgID: 0,
			State: types.AuthStateNil,
		}
	}

	tokenStr := extractTokenFromHeader(authHeader)
	if tokenStr == "" {
		logs.DebugContextw(ctx, "no valid token found in request", "authHeader", authHeader)
		return &types.Caller{
			Uin:   0,
			OrgID: 0,
			State: types.AuthStateNil,
		}
	}

	userCaller, userErr := parseUserCaller(ctx, tokenStr, jwtSecret, database)
	if userErr == nil {
		return userCaller
	}

	if workerCaller, workerErr := parseWorkerCaller(tokenStr, jwtSecret); workerErr == nil {
		return workerCaller
	} else {
		logs.WarnContextw(ctx, "parse auth token failed", "user_error", userErr, "worker_error", workerErr)
	}
	return failedCaller()
}

func parseUserCaller(ctx *gin.Context, tokenStr, jwtSecret string, database *gorm.DB) (*types.Caller, error) {
	claims, err := localauth.ParseUserToken(tokenStr, jwtSecret)
	if err != nil {
		return nil, err
	}

	queryCtx, cancel := context.WithTimeout(ctx.Request.Context(), 3*time.Second)
	defer cancel()

	userOrg, err := db.GetUserOrgByUin(queryCtx, database, claims.Uin)
	if err != nil {
		logs.WarnContextw(ctx, "get user org failed, db error", "error", err, "uin", claims.Uin)
		return &types.Caller{
			Uin:   claims.Uin,
			Kind:  types.CallerKindUser,
			State: types.AuthStateFailed,
		}, nil
	}
	if userOrg == nil {
		logs.WarnContextw(ctx, "user org not found", "uin", claims.Uin)
		return &types.Caller{
			Uin:   claims.Uin,
			Kind:  types.CallerKindUser,
			State: types.AuthStateFailed,
		}, nil
	}
	return &types.Caller{
		Uin:   userOrg.Uin,
		OrgID: userOrg.OrgID,
		Kind:  types.CallerKindUser,
		State: types.AuthStateSucc,
	}, nil
}

func parseWorkerCaller(tokenStr, jwtSecret string) (*types.Caller, error) {
	claims, err := localauth.ParseWorkerToken(tokenStr, jwtSecret)
	if err != nil {
		return nil, err
	}
	return &types.Caller{
		Uin:      0,
		OrgID:    claims.OrgID,
		WorkerID: claims.WorkerID,
		Kind:     types.CallerKindWorker,
		State:    types.AuthStateSucc,
	}, nil
}

func failedCaller() *types.Caller {
	return &types.Caller{
		Uin:   0,
		OrgID: 0,
		State: types.AuthStateFailed,
	}
}

func extractTokenFromHeader(authHeader string) string {
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	return strings.TrimSpace(authHeader)
}
