package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

type pluginHandlerTestService struct {
	listOrgID          uint
	listUin            uint
	statusOrgID        uint
	statusUin          uint
	statusKind         string
	statusCode         string
	downloadOrgID      uint
	downloadCallerKind types.CallerKind
	downloadCallerID   uint
	mcpOrgID           uint
	mcpUin             uint
	mcpPluginID        string
	mcpConfig          contract.MCPPluginConfig
	mcpTestURL         string
	platformOrgID      uint
	platformUin        uint
	platformCode       string
	platformAuthValues map[string]string
	oauthAttemptID     string
	oauthState         string
	oauthCode          string
}

func (s *pluginHandlerTestService) ListPlugins(_ context.Context, orgID, uin uint, req *contract.ListPluginsRequest) (*contract.ListPluginsResponse, error) {
	s.listOrgID, s.listUin = orgID, uin
	return &contract.ListPluginsResponse{Plugins: []contract.PluginView{}}, nil
}

func (*pluginHandlerTestService) GetPlugin(context.Context, uint, uint, string, *contract.GetPluginRequest) (*contract.GetPluginResponse, error) {
	return nil, contract.ErrPluginNotFound
}

func (s *pluginHandlerTestService) GetPluginInstallationStatus(
	_ context.Context,
	orgID, uin uint,
	req *contract.GetPluginInstallationStatusRequest,
) (*contract.PluginInstallationStatusResponse, error) {
	s.statusOrgID, s.statusUin, s.statusKind, s.statusCode = orgID, uin, req.Kind, req.Code
	return &contract.PluginInstallationStatusResponse{
		Kind: req.Kind, Code: req.Code,
	}, nil
}

func (*pluginHandlerTestService) ListPluginVersions(context.Context, uint, uint, string) (*contract.ListPluginVersionsResponse, error) {
	return &contract.ListPluginVersionsResponse{Versions: []contract.PluginRevisionView{}}, nil
}

func (*pluginHandlerTestService) DeletePlugin(context.Context, uint, uint, string, *contract.DeletePluginRequest) (*contract.DeletePluginResponse, error) {
	return &contract.DeletePluginResponse{Operation: "archived"}, nil
}

func (*pluginHandlerTestService) AddSkillPlugin(context.Context, uint, uint, *contract.AddSkillPluginRequest) error {
	return nil
}

func (s *pluginHandlerTestService) AddMCPPlugin(
	_ context.Context,
	orgID, uin uint,
	req *contract.AddMCPPluginRequest,
) (*contract.PluginView, error) {
	s.mcpOrgID, s.mcpUin, s.mcpConfig = orgID, uin, req.MCPPluginConfig
	return &contract.PluginView{PublicID: "plugin_mcp", Code: req.Code, Kind: "mcp"}, nil
}

func (s *pluginHandlerTestService) UpdateMCPPlugin(
	_ context.Context,
	orgID, uin uint,
	pluginID string,
	req *contract.UpdateMCPPluginRequest,
) (*contract.PluginView, error) {
	s.mcpOrgID, s.mcpUin, s.mcpPluginID, s.mcpConfig = orgID, uin, pluginID, req.MCPPluginConfig
	return &contract.PluginView{PublicID: pluginID, Code: req.Code, Kind: "mcp"}, nil
}

func (s *pluginHandlerTestService) TestMCPPlugin(
	_ context.Context,
	req *contract.TestMCPPluginRequest,
) (*contract.TestMCPPluginResponse, error) {
	s.mcpTestURL = req.URL
	return &contract.TestMCPPluginResponse{OK: true, ToolCount: 2}, nil
}

func (s *pluginHandlerTestService) ListMCPPlatforms(
	_ context.Context,
	orgID, uin uint,
) (*contract.ListMCPPlatformsResponse, error) {
	s.platformOrgID, s.platformUin = orgID, uin
	return &contract.ListMCPPlatformsResponse{Platforms: []contract.MCPPlatformView{
		{Code: "corekg", Name: "CoreKG"},
	}}, nil
}

func (s *pluginHandlerTestService) ConnectMCPPlatform(
	_ context.Context,
	orgID, uin uint,
	platformCode string,
	req *contract.ConnectMCPPlatformRequest,
) (*contract.ConnectMCPPlatformResponse, error) {
	s.platformOrgID, s.platformUin, s.platformCode = orgID, uin, platformCode
	s.platformAuthValues = req.AuthValues
	return &contract.ConnectMCPPlatformResponse{
		Platform:  contract.MCPPlatformView{Code: platformCode, Connected: true},
		Plugin:    contract.PluginView{PublicID: "plugin_corekg", Kind: "mcp"},
		ToolCount: 21,
	}, nil
}

func (s *pluginHandlerTestService) StartMCPPlatformOAuth(
	_ context.Context,
	orgID uint,
	uin uint,
	platformCode string,
) (*contract.StartMCPPlatformOAuthResponse, error) {
	s.platformOrgID, s.platformUin, s.platformCode = orgID, uin, platformCode
	return &contract.StartMCPPlatformOAuthResponse{
		AttemptID: "oauth_attempt", AuthorizationURL: "https://openapi.baidu.com/authorize",
	}, nil
}

func (s *pluginHandlerTestService) GetMCPPlatformOAuthStatus(
	_ context.Context,
	orgID uint,
	uin uint,
	platformCode string,
	attemptID string,
) (*contract.MCPPlatformOAuthStatusResponse, error) {
	s.platformOrgID, s.platformUin, s.platformCode, s.oauthAttemptID = orgID, uin, platformCode, attemptID
	return &contract.MCPPlatformOAuthStatusResponse{AttemptID: attemptID, Status: "pending"}, nil
}

func (s *pluginHandlerTestService) CompleteMCPPlatformOAuth(
	_ context.Context,
	platformCode string,
	state string,
	code string,
	_ string,
) (*contract.MCPPlatformOAuthStatusResponse, error) {
	s.platformCode, s.oauthState, s.oauthCode = platformCode, state, code
	return &contract.MCPPlatformOAuthStatusResponse{Status: "active", Connected: true}, nil
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

func TestPluginMCPRoutesPassCallerIdentityAndDraftConfig(t *testing.T) {
	service := &pluginHandlerTestService{}
	router := newPluginHandlerTestRouter(service)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/plugins/mcp",
		bytes.NewBufferString(
			`{"name":"SQLite","transport":"stdio","command":"npx",`+
				`"args":["-y","@example/mcp"],"env":{"LOG_LEVEL":"debug"}}`,
		),
	)
	createRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d", createRecorder.Code, http.StatusOK)
	}
	if service.mcpOrgID != 42 || service.mcpUin != 7 || service.mcpConfig.Code != "" ||
		service.mcpConfig.Transport != "stdio" || service.mcpConfig.Command != "npx" ||
		service.mcpConfig.Env["LOG_LEVEL"] != "debug" {
		t.Fatalf("create caller/config = org=%d uin=%d config=%#v", service.mcpOrgID, service.mcpUin, service.mcpConfig)
	}

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(
		http.MethodPut,
		"/plugins/mcp/plugin_mcp",
		bytes.NewBufferString(`{"name":"Docs v2","url":"https://example.com/v2/mcp"}`),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK || service.mcpPluginID != "plugin_mcp" {
		t.Fatalf("update status/plugin = %d/%q", updateRecorder.Code, service.mcpPluginID)
	}

	testRecorder := httptest.NewRecorder()
	testRequest := httptest.NewRequest(
		http.MethodPost,
		"/plugins/mcp/test",
		bytes.NewBufferString(`{"url":"https://example.com/test-mcp"}`),
	)
	testRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(testRecorder, testRequest)
	if testRecorder.Code != http.StatusOK || service.mcpTestURL != "https://example.com/test-mcp" {
		t.Fatalf("test status/url = %d/%q", testRecorder.Code, service.mcpTestURL)
	}
}

func TestPluginMCPPlatformRoutesPassCallerIdentity(t *testing.T) {
	service := &pluginHandlerTestService{}
	router := newPluginHandlerTestRouter(service)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/plugins/mcp/platforms", nil)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK || service.platformOrgID != 42 || service.platformUin != 7 {
		t.Fatalf(
			"list status/caller = %d/%d/%d",
			listRecorder.Code,
			service.platformOrgID,
			service.platformUin,
		)
	}

	connectRecorder := httptest.NewRecorder()
	connectRequest := httptest.NewRequest(
		http.MethodPost,
		"/plugins/mcp/platforms/netease-mail/connect",
		bytes.NewBufferString(`{"auth_values":{"email":"user@example.com"}}`),
	)
	connectRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(connectRecorder, connectRequest)
	if connectRecorder.Code != http.StatusOK ||
		service.platformOrgID != 42 ||
		service.platformUin != 7 ||
		service.platformCode != "netease-mail" ||
		service.platformAuthValues["email"] != "user@example.com" {
		t.Fatalf(
			"connect status/caller/platform = %d/%d/%d/%q",
			connectRecorder.Code,
			service.platformOrgID,
			service.platformUin,
			service.platformCode,
		)
	}
}

func TestPluginMCPPlatformOAuthRoutesPassCallerIdentity(t *testing.T) {
	service := &pluginHandlerTestService{}
	router := newPluginHandlerTestRouter(service)

	startRecorder := httptest.NewRecorder()
	router.ServeHTTP(
		startRecorder,
		httptest.NewRequest(http.MethodPost, "/plugins/mcp/platforms/baidu-netdisk/oauth/start", nil),
	)
	if startRecorder.Code != http.StatusOK || service.platformOrgID != 42 || service.platformUin != 7 ||
		service.platformCode != "baidu-netdisk" {
		t.Fatalf(
			"start status/caller/platform = %d/%d/%d/%q",
			startRecorder.Code,
			service.platformOrgID,
			service.platformUin,
			service.platformCode,
		)
	}

	statusRecorder := httptest.NewRecorder()
	router.ServeHTTP(
		statusRecorder,
		httptest.NewRequest(
			http.MethodGet,
			"/plugins/mcp/platforms/baidu-netdisk/oauth/status?attempt_id=oauth_attempt",
			nil,
		),
	)
	if statusRecorder.Code != http.StatusOK || service.oauthAttemptID != "oauth_attempt" {
		t.Fatalf("status response/attempt = %d/%q", statusRecorder.Code, service.oauthAttemptID)
	}
}

func TestPluginMCPPlatformOAuthCallbackReturnsSafeStaticPage(t *testing.T) {
	service := &pluginHandlerTestService{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPluginOAuthCallbackRoutes(router, service)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/plugins/mcp/oauth/baidu-netdisk/callback?state=plugin.secret-state&code=provider-code",
			nil,
		),
	)
	if recorder.Code != http.StatusOK || service.platformCode != "baidu-netdisk" ||
		service.oauthState != "plugin.secret-state" || service.oauthCode != "provider-code" {
		t.Fatalf(
			"callback status/platform/state/code = %d/%q/%q/%q",
			recorder.Code,
			service.platformCode,
			service.oauthState,
			service.oauthCode,
		)
	}
	if strings.Contains(recorder.Body.String(), "secret-state") ||
		strings.Contains(recorder.Body.String(), "provider-code") {
		t.Fatalf("callback page leaked query values: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "返回 Lework") ||
		strings.Contains(recorder.Body.String(), "SingerOS") {
		t.Fatalf("callback page uses unexpected product name: %s", recorder.Body.String())
	}
	if recorder.Header().Get("Referrer-Policy") != "no-referrer" ||
		recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("callback security headers = %#v", recorder.Header())
	}
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
	if service.listOrgID != 42 || service.listUin != 7 {
		t.Fatalf("list caller = (%d, %d), want (42, 7)", service.listOrgID, service.listUin)
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
	if service.statusOrgID != 42 || service.statusUin != 7 || service.statusKind != "skill" ||
		service.statusCode != "official-skill" {
		t.Fatalf(
			"installation status query = org=%d uin=%d kind=%q code=%q",
			service.statusOrgID,
			service.statusUin,
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
