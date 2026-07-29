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
	listOrgID          uint
	statusOrgID        uint
	statusKind         string
	statusCode         string
	downloadOrgID      uint
	downloadCallerKind types.CallerKind
	downloadCallerID   uint
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

func (s *pluginHandlerTestService) ResolveSkillDownloadURLs(
	_ context.Context,
	orgID uint,
	callerKind types.CallerKind,
	callerID uint,
	_ *contract.ResolveSkillDownloadURLsRequest,
) (*contract.ResolveSkillDownloadURLsResponse, error) {
	s.downloadOrgID = orgID
	s.downloadCallerKind = callerKind
	s.downloadCallerID = callerID
	return &contract.ResolveSkillDownloadURLsResponse{Skills: []contract.SkillDownloadURL{}}, nil
}

func (*pluginHandlerTestService) ListBuiltinSkills(context.Context) (*contract.ListPluginsResponse, error) {
	return &contract.ListPluginsResponse{Plugins: []contract.PluginView{}}, nil
}

func newPluginHandlerTestRouter(service contract.PluginService) *gin.Engine {
	return newPluginHandlerTestRouterWithCaller(
		service,
		&types.Caller{
			OrgID: 42,
			Uin:   7,
			Kind:  types.CallerKindUser,
			State: types.AuthStateSucc,
		},
	)
}

func newPluginHandlerTestRouterWithCaller(
	service contract.PluginService,
	caller *types.Caller,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		auth.WithGinContext(ctx, caller, nil, "")
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
	service := &pluginHandlerTestService{}
	router := newPluginHandlerTestRouter(service)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugins/skills/download-urls", bytes.NewBufferString(`{"skill_codes":["xlsx"]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.downloadOrgID != 42 ||
		service.downloadCallerKind != types.CallerKindUser ||
		service.downloadCallerID != 7 {
		t.Fatalf(
			"user download caller = org=%d kind=%q id=%d",
			service.downloadOrgID,
			service.downloadCallerKind,
			service.downloadCallerID,
		)
	}

	workerService := &pluginHandlerTestService{}
	workerRouter := newPluginHandlerTestRouterWithCaller(
		workerService,
		&types.Caller{
			OrgID:    43,
			WorkerID: 19,
			Kind:     types.CallerKindWorker,
			State:    types.AuthStateSucc,
		},
	)
	workerRecorder := httptest.NewRecorder()
	workerRequest := httptest.NewRequest(
		http.MethodPost,
		"/plugins/skills/download-urls",
		bytes.NewBufferString(`{"skill_codes":["xlsx"]}`),
	)
	workerRequest.Header.Set("Content-Type", "application/json")
	workerRouter.ServeHTTP(workerRecorder, workerRequest)
	if workerRecorder.Code != http.StatusOK {
		t.Fatalf("worker status = %d, want %d", workerRecorder.Code, http.StatusOK)
	}
	if workerService.downloadOrgID != 43 ||
		workerService.downloadCallerKind != types.CallerKindWorker ||
		workerService.downloadCallerID != 19 {
		t.Fatalf(
			"worker download caller = org=%d kind=%q id=%d",
			workerService.downloadOrgID,
			workerService.downloadCallerKind,
			workerService.downloadCallerID,
		)
	}
}
