package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/insmtx/Leros/backend/internal/adapter/account"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type mcpPlatformAPIKeyIssuer struct {
	input account.CreateAPIKeyInput
	calls int
}

func (i *mcpPlatformAPIKeyIssuer) CreateAPIKey(
	_ context.Context,
	input account.CreateAPIKeyInput,
) (*account.CreatedAPIKey, error) {
	i.input = input
	i.calls++
	return &account.CreatedAPIKey{ID: 9, APIKey: "yg-corekg-test"}, nil
}

func TestCoreKGMCPPlatformConnectIsUserScopedAndIdempotent(t *testing.T) {
	server := mcpserver.NewMCPServer("corekg-test", "1.0.0")
	server.AddTool(
		mcpsdk.NewTool("search", mcpsdk.WithDescription("Search knowledge")),
		func(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return mcpsdk.NewToolResultText("ok"), nil
		},
	)
	streamableServer := mcpserver.NewStreamableHTTPServer(server)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer yg-corekg-test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		streamableServer.ServeHTTP(w, req)
	}))
	defer httpServer.Close()

	database := setupPluginServiceTestDB(t)
	issuer := &mcpPlatformAPIKeyIssuer{}
	service := &pluginService{db: database, apiKeyIssuer: issuer, coreKGMCPURL: httpServer.URL}

	before, err := service.ListMCPPlatforms(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("ListMCPPlatforms() before connect error = %v", err)
	}
	if len(before.Platforms) != 1 || before.Platforms[0].Connected || !before.Platforms[0].AutoConnectSupported {
		t.Fatalf("platform before connect = %#v", before.Platforms)
	}

	connected, err := service.ConnectMCPPlatform(context.Background(), 10, 20, "corekg")
	if err != nil {
		t.Fatalf("ConnectMCPPlatform() error = %v", err)
	}
	if !connected.Platform.Connected || connected.ToolCount != 1 || connected.Plugin.PublicID == "" {
		t.Fatalf("connected response = %#v", connected)
	}
	if issuer.calls != 1 ||
		issuer.input.Name != "SingerOS CoreKG MCP" ||
		issuer.input.Purpose != "mcp_connector" ||
		issuer.input.ResourceType != "mcp" ||
		issuer.input.ResourceID != 0 ||
		issuer.input.ExpireHours != 0 {
		t.Fatalf("issuer calls/input = %d/%#v", issuer.calls, issuer.input)
	}

	repeated, err := service.ConnectMCPPlatform(context.Background(), 10, 20, "COREKG")
	if err != nil {
		t.Fatalf("repeated ConnectMCPPlatform() error = %v", err)
	}
	if repeated.Plugin.PublicID != connected.Plugin.PublicID || issuer.calls != 1 {
		t.Fatalf("repeated response/calls = %#v/%d", repeated, issuer.calls)
	}

	plugin, err := infradb.GetPluginByPublicID(context.Background(), database, 10, connected.Plugin.PublicID)
	if err != nil || plugin == nil {
		t.Fatalf("GetPluginByPublicID() plugin/error = %#v/%v", plugin, err)
	}
	revision, err := infradb.GetCurrentPluginRevision(context.Background(), database, plugin)
	if err != nil || revision == nil {
		t.Fatalf("GetCurrentPluginRevision() revision/error = %#v/%v", revision, err)
	}
	definition, err := MCPFromDefinition(revision.Definition)
	if err != nil {
		t.Fatalf("MCPFromDefinition() error = %v", err)
	}
	if definition.Provider != "corekg" ||
		definition.URL != httpServer.URL ||
		definition.BearerToken != "yg-corekg-test" {
		t.Fatalf("CoreKG definition = %#v", definition)
	}

	otherUser, err := service.ListMCPPlatforms(context.Background(), 10, 21)
	if err != nil {
		t.Fatalf("other user ListMCPPlatforms() error = %v", err)
	}
	if otherUser.Platforms[0].Connected {
		t.Fatalf("other user platform = %#v", otherUser.Platforms[0])
	}
}

func TestMCPListEnsuresCoreKGConnection(t *testing.T) {
	server := mcpserver.NewMCPServer("corekg-list-test", "1.0.0")
	server.AddTool(
		mcpsdk.NewTool("search", mcpsdk.WithDescription("Search knowledge")),
		func(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return mcpsdk.NewToolResultText("ok"), nil
		},
	)
	streamableServer := mcpserver.NewStreamableHTTPServer(server)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		streamableServer.ServeHTTP(w, req)
	}))
	defer httpServer.Close()

	issuer := &mcpPlatformAPIKeyIssuer{}
	service := &pluginService{
		db:           setupPluginServiceTestDB(t),
		apiKeyIssuer: issuer,
		coreKGMCPURL: httpServer.URL,
	}
	request := &contract.ListPluginsRequest{Kind: "mcp", Status: "active"}
	first, err := service.ListPlugins(context.Background(), 10, 20, request)
	if err != nil {
		t.Fatalf("ListPlugins() first error = %v", err)
	}
	if len(first.Plugins) != 1 || first.Plugins[0].Code != coreKGPluginCode(10, 20) {
		t.Fatalf("first plugins = %#v", first.Plugins)
	}
	second, err := service.ListPlugins(context.Background(), 10, 20, request)
	if err != nil {
		t.Fatalf("ListPlugins() second error = %v", err)
	}
	if len(second.Plugins) != 1 || issuer.calls != 1 {
		t.Fatalf("second plugins/issuer calls = %#v/%d", second.Plugins, issuer.calls)
	}
}

func TestCoreKGMCPPlatformIsUnavailableWithoutIAMIssuer(t *testing.T) {
	service := NewPluginService(setupPluginServiceTestDB(t))
	platforms, err := service.ListMCPPlatforms(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("ListMCPPlatforms() error = %v", err)
	}
	if platforms.Platforms[0].AutoConnectSupported {
		t.Fatalf("platform = %#v", platforms.Platforms[0])
	}
	if _, err := service.ConnectMCPPlatform(context.Background(), 10, 20, "corekg"); err == nil {
		t.Fatal("ConnectMCPPlatform() expected unsupported error")
	}
}

func TestCoreKGMCPURLUsesConfiguredAuthBase(t *testing.T) {
	for name, testCase := range map[string]struct {
		baseURL string
		want    string
	}{
		"plain":    {baseURL: "https://tapi.yygu.cn", want: "https://tapi.yygu.cn/v3/keapi/mcp"},
		"trailing": {baseURL: "https://tapi.yygu.cn/", want: "https://tapi.yygu.cn/v3/keapi/mcp"},
		"empty":    {baseURL: " ", want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := coreKGMCPURLFromAuthBase(testCase.baseURL); got != testCase.want {
				t.Fatalf("coreKGMCPURLFromAuthBase(%q) = %q, want %q", testCase.baseURL, got, testCase.want)
			}
		})
	}
}

func TestCoreKGProviderSurvivesMCPUpdate(t *testing.T) {
	database := setupPluginServiceTestDB(t)
	service := NewPluginService(database)
	created, err := service.AddMCPPlugin(context.Background(), 10, 20, &contract.AddMCPPluginRequest{
		MCPPluginConfig: contract.MCPPluginConfig{
			Code:     coreKGPluginCode(10, 20),
			Name:     "CoreKG",
			URL:      "https://example.com/mcp",
			Provider: "corekg",
		},
	})
	if err != nil {
		t.Fatalf("AddMCPPlugin() error = %v", err)
	}
	if _, err := service.UpdateMCPPlugin(context.Background(), 10, 20, created.PublicID, &contract.UpdateMCPPluginRequest{
		MCPPluginConfig: contract.MCPPluginConfig{
			Name: "CoreKG renamed",
			URL:  "https://example.com/mcp-v2",
		},
	}); err != nil {
		t.Fatalf("UpdateMCPPlugin() error = %v", err)
	}
	detail, err := service.GetPlugin(
		context.Background(),
		10,
		20,
		created.PublicID,
		&contract.GetPluginRequest{},
	)
	if err != nil {
		t.Fatalf("GetPlugin() error = %v", err)
	}
	definition, err := MCPFromDefinition(detail.Definition)
	if err != nil || definition.Provider != "corekg" {
		t.Fatalf("updated definition/error = %#v/%v", definition, err)
	}
}
