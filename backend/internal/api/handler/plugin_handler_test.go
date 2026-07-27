package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

type pluginHandlerTestService struct {
	listOrgID   uint
	statusOrgID uint
	statusKind  string
	statusCode  string
}

func (s *pluginHandlerTestService) ListPlugins(_ context.Context, orgID uint, req *contract.ListPluginsRequest) (*contract.ListPluginsResponse, error) {
	s.listOrgID = orgID
	return &contract.ListPluginsResponse{Plugins: []contract.PluginView{}}, nil
}

func (*pluginHandlerTestService) GetPlugin(context.Context, uint, string, *contract.GetPluginRequest) (*contract.GetPluginResponse, error) {
	return nil, contract.ErrPluginNotFound
}

func (s *pluginHandlerTestService) GetPluginInstallationStatus(
	_ context.Context,
	orgID uint,
	req *contract.GetPluginInstallationStatusRequest,
) (*contract.PluginInstallationStatusResponse, error) {
	s.statusOrgID, s.statusKind, s.statusCode = orgID, req.Kind, req.Code
	return &contract.PluginInstallationStatusResponse{
		Kind: req.Kind, Code: req.Code,
	}, nil
}

func (*pluginHandlerTestService) ListPluginVersions(context.Context, uint, string) (*contract.ListPluginVersionsResponse, error) {
	return &contract.ListPluginVersionsResponse{Versions: []contract.PluginRevisionView{}}, nil
}

func (*pluginHandlerTestService) DeletePlugin(context.Context, uint, uint, string, *contract.DeletePluginRequest) (*contract.DeletePluginResponse, error) {
	return &contract.DeletePluginResponse{Operation: "archived"}, nil
}

func (*pluginHandlerTestService) AddSkillPlugin(context.Context, uint, uint, *contract.AddSkillPluginRequest) error {
	return nil
}

func (*pluginHandlerTestService) ResolveSkillDownloadURLs(context.Context, uint, *contract.ResolveSkillDownloadURLsRequest) (*contract.ResolveSkillDownloadURLsResponse, error) {
	return &contract.ResolveSkillDownloadURLsResponse{Skills: []contract.SkillDownloadURL{}}, nil
}

func newPluginHandlerTestRouter(service contract.PluginService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		auth.WithGinContext(ctx, &types.Caller{OrgID: 42, Uin: 7, State: types.AuthStateSucc}, nil, "")
		ctx.Next()
	})
	RegisterPluginRoutes(router, service)
	return router
}

func TestPluginRoutesUseCallerOrganizationAndExposeNotFound(t *testing.T) {
	service := &pluginHandlerTestService{}
	router := newPluginHandlerTestRouter(service)

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/plugins?scope=organization", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRecorder.Code, http.StatusOK)
	}
	if service.listOrgID != 42 {
		t.Fatalf("list org = %d, want 42", service.listOrgID)
	}

	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, httptest.NewRequest(http.MethodGet, "/plugins/plg_missing", nil))
	if detailRecorder.Code != http.StatusNotFound {
		t.Fatalf("detail status = %d, want %d", detailRecorder.Code, http.StatusNotFound)
	}
}

func TestPluginSkillImportValidatesRequest(t *testing.T) {
	router := newPluginHandlerTestRouter(&pluginHandlerTestService{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugins/skills", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 40001 {
		t.Fatalf("code = %d, want 40001", body.Code)
	}
}

func TestPluginInstallationStatusUsesCallerOrganizationAndValidatesIdentity(t *testing.T) {
	service := &pluginHandlerTestService{}
	router := newPluginHandlerTestRouter(service)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/plugins/installation-status?kind=skill&code=official-skill",
			nil,
		),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.statusOrgID != 42 || service.statusKind != "skill" ||
		service.statusCode != "official-skill" {
		t.Fatalf(
			"installation status query = org=%d kind=%q code=%q",
			service.statusOrgID,
			service.statusKind,
			service.statusCode,
		)
	}

	missingRecorder := httptest.NewRecorder()
	router.ServeHTTP(
		missingRecorder,
		httptest.NewRequest(http.MethodGet, "/plugins/installation-status?kind=skill", nil),
	)
	if missingRecorder.Code != http.StatusBadRequest {
		t.Fatalf("missing code status = %d, want %d", missingRecorder.Code, http.StatusBadRequest)
	}
}

func TestPluginSkillDownloadURLsAcceptsCodeArray(t *testing.T) {
	router := newPluginHandlerTestRouter(&pluginHandlerTestService{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugins/skills/download-urls", bytes.NewBufferString(`{"skill_codes":["xlsx"]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
