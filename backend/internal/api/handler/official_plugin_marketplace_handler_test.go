package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/api/auth"
	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

type officialPluginMarketplaceHandlerTestService struct {
	installOrgID uint
	installUIN   uint
	installID    string
	latestKind   string
	latestCode   string
}

func (*officialPluginMarketplaceHandlerTestService) ListOfficialPluginMarketplaceItems(context.Context, *contract.ListOfficialPluginMarketplaceItemsRequest) (*contract.ListOfficialPluginMarketplaceItemsResponse, error) {
	return &contract.ListOfficialPluginMarketplaceItemsResponse{Items: []contract.OfficialPluginMarketplaceItemView{}}, nil
}

func (*officialPluginMarketplaceHandlerTestService) GetOfficialPluginMarketplaceItem(context.Context, string) (*contract.OfficialPluginMarketplaceItemView, error) {
	return nil, contract.ErrPluginNotFound
}

func (s *officialPluginMarketplaceHandlerTestService) GetOfficialPluginLatestVersion(
	_ context.Context,
	req *contract.GetOfficialPluginLatestVersionRequest,
) (*contract.OfficialPluginLatestVersionResponse, error) {
	s.latestKind, s.latestCode = req.Kind, req.Code
	return &contract.OfficialPluginLatestVersionResponse{
		Kind: req.Kind, Code: req.Code,
	}, nil
}

func (s *officialPluginMarketplaceHandlerTestService) InstallOfficialPlugin(_ context.Context, orgID, uin uint, itemID string) (*contract.InstallOfficialPluginResponse, error) {
	s.installOrgID, s.installUIN, s.installID = orgID, uin, itemID
	return &contract.InstallOfficialPluginResponse{Operation: "installed", Plugin: contract.PluginView{}}, nil
}

func TestOfficialPluginMarketplaceInstallUsesCallerOrganization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &officialPluginMarketplaceHandlerTestService{}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		auth.WithGinContext(ctx, &types.Caller{OrgID: 42, Uin: 7, State: types.AuthStateSucc}, nil, "")
		ctx.Next()
	})
	RegisterOfficialPluginMarketplaceRoutes(router, service)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/plugin-marketplace/items/mkt_official/install", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.installOrgID != 42 || service.installUIN != 7 || service.installID != "mkt_official" {
		t.Fatalf("install caller = org=%d uin=%d item=%q", service.installOrgID, service.installUIN, service.installID)
	}
}

func TestOfficialPluginLatestVersionUsesStaticRouteAndValidatesIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &officialPluginMarketplaceHandlerTestService{}
	router := gin.New()
	RegisterOfficialPluginMarketplaceRoutes(router, service)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/plugin-marketplace/items/latest-version?kind=skill&code=official-skill",
			nil,
		),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.latestKind != "skill" || service.latestCode != "official-skill" {
		t.Fatalf("latest version query = kind=%q code=%q", service.latestKind, service.latestCode)
	}

	missingRecorder := httptest.NewRecorder()
	router.ServeHTTP(
		missingRecorder,
		httptest.NewRequest(
			http.MethodGet,
			"/plugin-marketplace/items/latest-version?kind=skill",
			nil,
		),
	)
	if missingRecorder.Code != http.StatusBadRequest {
		t.Fatalf("missing code status = %d, want %d", missingRecorder.Code, http.StatusBadRequest)
	}
}
