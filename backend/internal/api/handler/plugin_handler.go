package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/api/dto"
	"github.com/insmtx/Leros/backend/types"
)

// RegisterPluginRoutes registers organization plugin repository routes.
func RegisterPluginRoutes(r gin.IRouter, service contract.PluginService) {
	r.GET("/plugins", listPlugins(service))
	r.GET("/plugins/installation-status", getPluginInstallationStatus(service))
	r.POST("/plugins/skills", addSkillPlugin(service))
	r.POST("/plugins/skills/download-urls", resolveSkillDownloadURLs(service))
	r.GET("/plugins/builtin-skills", listBuiltinSkills(service))
	r.DELETE("/plugins/:plugin_id", deletePlugin(service))
	r.GET("/plugins/:plugin_id", getPlugin(service))
	r.GET("/plugins/:plugin_id/versions", listPluginVersions(service))
}

func resolveSkillDownloadURLs(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		caller, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		var req contract.ResolveSkillDownloadURLsRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, err.Error()))
			return
		}
		result, err := service.ResolveSkillDownloadURLs(ctx, caller.OrgID, caller.Kind, callerID(caller), &req)
		writePluginServiceResult(ctx, result, err)
	}
}

func getPluginInstallationStatus(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		caller, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		var req contract.GetPluginInstallationStatusRequest
		if err := ctx.ShouldBindQuery(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, err.Error()))
			return
		}
		if strings.TrimSpace(req.Kind) == "" || strings.TrimSpace(req.Code) == "" {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, "kind and code are required"))
			return
		}
		result, err := service.GetPluginInstallationStatus(ctx, caller.OrgID, &req)
		writePluginServiceResult(ctx, result, err)
	}
}

func listPlugins(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		caller, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		var req contract.ListPluginsRequest
		if err := ctx.ShouldBindQuery(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, err.Error()))
			return
		}
		result, err := service.ListPlugins(ctx, caller.OrgID, &req)
		writePluginServiceResult(ctx, result, err)
	}
}

func getPlugin(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		caller, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		pluginID := strings.TrimSpace(ctx.Param("plugin_id"))
		if pluginID == "" {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, "plugin_id is required"))
			return
		}
		var req contract.GetPluginRequest
		if err := ctx.ShouldBindQuery(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, err.Error()))
			return
		}
		result, err := service.GetPlugin(ctx, caller.OrgID, pluginID, &req)
		writePluginServiceResult(ctx, result, err)
	}
}

func listPluginVersions(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		caller, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		pluginID := strings.TrimSpace(ctx.Param("plugin_id"))
		if pluginID == "" {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, "plugin_id is required"))
			return
		}
		result, err := service.ListPluginVersions(ctx, caller.OrgID, pluginID)
		writePluginServiceResult(ctx, result, err)
	}
}

func deletePlugin(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		caller, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		pluginID := strings.TrimSpace(ctx.Param("plugin_id"))
		if pluginID == "" {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, "plugin_id is required"))
			return
		}
		var req contract.DeletePluginRequest
		if err := ctx.ShouldBindQuery(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, err.Error()))
			return
		}
		result, err := service.DeletePlugin(ctx, caller.OrgID, caller.Uin, pluginID, &req)
		writePluginServiceResult(ctx, result, err)
	}
}

func addSkillPlugin(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		caller, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		var req contract.AddSkillPluginRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, err.Error()))
			return
		}
		switch strings.TrimSpace(req.Mode) {
		case contract.SkillAddModeFile:
			if strings.TrimSpace(req.FileUploadID) == "" || strings.TrimSpace(req.GitHubURL) != "" {
				ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, "file mode requires file_upload_id only"))
				return
			}
		case contract.SkillAddModeGitHub:
			if strings.TrimSpace(req.GitHubURL) == "" || strings.TrimSpace(req.FileUploadID) != "" {
				ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, "github mode requires github_url only"))
				return
			}
		default:
			ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeInvalidParams, "mode must be file or github"))
			return
		}
		err := service.AddSkillPlugin(ctx, caller.OrgID, caller.Uin, &req)
		writePluginServiceResult(ctx, nil, err)
	}
}

func pluginCaller(ctx *gin.Context) (*types.Caller, bool) {
	caller, _ := auth.FromGinContext(ctx)
	if caller == nil || caller.OrgID == 0 {
		ctx.JSON(http.StatusUnauthorized, dto.Error(dto.CodeUnauthorized, "organization identity is required"))
		return nil, false
	}
	return caller, true
}

func callerID(caller *types.Caller) uint {
	if caller.Kind == types.CallerKindWorker {
		return caller.WorkerID
	}
	return caller.Uin
}

func writePluginServiceResult(ctx *gin.Context, result interface{}, err error) {
	if err == nil {
		ctx.JSON(http.StatusOK, dto.Success(result))
		return
	}
	if errors.Is(err, contract.ErrPluginNotFound) {
		ctx.JSON(http.StatusNotFound, dto.Error(dto.CodeNotFound, err.Error()))
		return
	}
	if strings.Contains(err.Error(), "definition") || strings.Contains(err.Error(), "plugin kind") {
		ctx.JSON(http.StatusBadRequest, dto.Error(dto.CodeValidationError, err.Error()))
		return
	}
	ctx.JSON(http.StatusInternalServerError, dto.Error(dto.CodeInternalError, err.Error()))
}

func listBuiltinSkills(service contract.PluginService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		_, ok := pluginCaller(ctx)
		if !ok {
			return
		}
		result, err := service.ListBuiltinSkills(ctx)
		writePluginServiceResult(ctx, result, err)
	}
}
