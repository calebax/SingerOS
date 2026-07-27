package contract

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrPluginNotFound indicates a plugin resource is not visible in the requested scope.
	ErrPluginNotFound = errors.New("plugin not found")
	// ErrPluginImportNotImplemented indicates that package publication is deferred to a later phase.
	ErrPluginImportNotImplemented = errors.New("plugin import is not implemented")
)

const (
	PluginScopeOrganization = "organization"
)

// ListPluginsRequest describes filters for organization plugin lists.
type ListPluginsRequest struct {
	Kind     string `form:"kind" json:"kind,omitempty"`
	Status   string `form:"status" json:"status,omitempty"`
	Category string `form:"category" json:"category,omitempty"`
	Keyword  string `form:"keyword" json:"keyword,omitempty"`
	Limit    int    `form:"limit" json:"limit,omitempty"`
}

// PluginView is the safe API representation of an organization plugin.
type PluginView struct {
	PublicID        string `json:"public_id"`
	Code            string `json:"code"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Status          string `json:"status"`
	Origin          string `json:"origin"`
	CurrentRevision int    `json:"current_revision"`
}

// ListPluginsResponse contains organization plugins.
type ListPluginsResponse struct {
	Plugins []PluginView `json:"plugins"`
}

// GetPluginRequest selects an organization plugin by public ID.
type GetPluginRequest struct{}

// GetPluginResponse contains one organization plugin detail.
type GetPluginResponse struct {
	Plugin  *PluginView                `json:"plugin,omitempty"`
	Content *PluginRevisionContentView `json:"content"`
}

// GetPluginInstallationStatusRequest identifies one organization plugin by stable identity.
type GetPluginInstallationStatusRequest struct {
	Kind string `form:"kind" json:"kind"`
	Code string `form:"code" json:"code"`
}

// PluginInstallationStatusResponse describes installation and official update state.
type PluginInstallationStatusResponse struct {
	Kind                        string `json:"kind"`
	Code                        string `json:"code"`
	Installed                   bool   `json:"installed"`
	PluginID                    string `json:"plugin_id,omitempty"`
	CurrentVersion              string `json:"current_version,omitempty"`
	MarketplaceBased            bool   `json:"marketplace_based"`
	MarketplaceItemID           string `json:"marketplace_item_id,omitempty"`
	InstalledMarketplaceVersion string `json:"installed_marketplace_version,omitempty"`
	MarketplaceAvailable        bool   `json:"marketplace_available"`
	LatestMarketplaceVersion    string `json:"latest_marketplace_version,omitempty"`
	UpdateAvailable             bool   `json:"update_available"`
}

// PluginRevisionFileView exposes one immutable file in the current plugin revision.
type PluginRevisionFileView struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

// PluginRevisionContentView contains the current revision content used by detail pages.
type PluginRevisionContentView struct {
	Schema         string                   `json:"schema"`
	Version        int                      `json:"version"`
	EntrypointPath string                   `json:"entrypoint_path"`
	SkillMD        string                   `json:"skill_md"`
	Files          []PluginRevisionFileView `json:"files"`
}

// PluginRevisionView exposes immutable revision metadata without the storage URI.
type PluginRevisionView struct {
	Revision        int       `json:"revision"`
	Status          string    `json:"status"`
	PublishedByType string    `json:"published_by_type"`
	PublishedByID   uint      `json:"published_by_id"`
	PublishedAt     time.Time `json:"published_at"`
}

// ListPluginVersionsResponse contains organization plugin revisions.
type ListPluginVersionsResponse struct {
	Versions []PluginRevisionView `json:"versions"`
}

// DeletePluginRequest optionally removes a plugin from one project instead of archiving it.
type DeletePluginRequest struct {
	ProjectID string `form:"project_id" json:"project_id,omitempty"`
}

// DeletePluginResponse reports which operation completed.
type DeletePluginResponse struct {
	Operation string `json:"operation"`
}

const (
	SkillAddModeFile   = "file"
	SkillAddModeGitHub = "github"
)

// AddSkillPluginRequest adds a Skill plugin using the selected source mode.
type AddSkillPluginRequest struct {
	Mode         string `json:"mode"`
	FileUploadID string `json:"file_upload_id"`
	GitHubURL    string `json:"github_url"`
}

// ResolveSkillDownloadURLsRequest selects the current downloadable artifacts by Skill code.
// Codes that are unavailable to the caller are omitted from the response.
type ResolveSkillDownloadURLsRequest struct {
	SkillCodes []string `json:"skill_codes"`
}

// SkillDownloadURL is the worker-safe projection of one current Skill artifact.
type SkillDownloadURL struct {
	Code        string `json:"code"`
	Revision    int    `json:"revision"`
	SHA256      string `json:"sha256"`
	DownloadURL string `json:"download_url"`
}

// OfficialPluginMarketplaceItemView is the public projection of one official plugin.
type OfficialPluginMarketplaceItemView struct {
	PublicID    string                     `json:"public_id"`
	Code        string                     `json:"code"`
	Kind        string                     `json:"kind"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Author      string                     `json:"author"`
	Version     string                     `json:"version"`
	Category    string                     `json:"category"`
	Tags        []string                   `json:"tags"`
	Icon        string                     `json:"icon,omitempty"`
	Verified    bool                       `json:"verified"`
	Content     *PluginRevisionContentView `json:"content,omitempty"`
}

// ListOfficialPluginMarketplaceItemsRequest filters the official plugin catalogue.
type ListOfficialPluginMarketplaceItemsRequest struct {
	Kind     string `form:"kind" json:"kind,omitempty"`
	Category string `form:"category" json:"category,omitempty"`
	Keyword  string `form:"keyword" json:"keyword,omitempty"`
	Limit    int    `form:"limit" json:"limit,omitempty"`
}

type ListOfficialPluginMarketplaceItemsResponse struct {
	Items []OfficialPluginMarketplaceItemView `json:"items"`
}

// GetOfficialPluginLatestVersionRequest identifies one official plugin by stable identity.
type GetOfficialPluginLatestVersionRequest struct {
	Kind string `form:"kind" json:"kind"`
	Code string `form:"code" json:"code"`
}

// OfficialPluginLatestVersionResponse reports whether an official release is available.
type OfficialPluginLatestVersionResponse struct {
	Kind          string `json:"kind"`
	Code          string `json:"code"`
	Available     bool   `json:"available"`
	ItemID        string `json:"item_id,omitempty"`
	LatestVersion string `json:"latest_version,omitempty"`
}

type InstallOfficialPluginResponse struct {
	Operation string     `json:"operation"`
	Plugin    PluginView `json:"plugin"`
}

// OfficialPluginMarketplaceService isolates official catalogue reads and installs from organization plugin APIs.
type OfficialPluginMarketplaceService interface {
	ListOfficialPluginMarketplaceItems(ctx context.Context, req *ListOfficialPluginMarketplaceItemsRequest) (*ListOfficialPluginMarketplaceItemsResponse, error)
	GetOfficialPluginMarketplaceItem(ctx context.Context, itemID string) (*OfficialPluginMarketplaceItemView, error)
	GetOfficialPluginLatestVersion(ctx context.Context, req *GetOfficialPluginLatestVersionRequest) (*OfficialPluginLatestVersionResponse, error)
	InstallOfficialPlugin(ctx context.Context, orgID, uin uint, itemID string) (*InstallOfficialPluginResponse, error)
}

// ResolveSkillDownloadURLsResponse contains only the Skill artifacts that could be resolved.
type ResolveSkillDownloadURLsResponse struct {
	Skills []SkillDownloadURL `json:"skills"`
}

// PluginService defines the new organization plugin management contract.
type PluginService interface {
	ListPlugins(ctx context.Context, orgID uint, req *ListPluginsRequest) (*ListPluginsResponse, error)
	GetPlugin(ctx context.Context, orgID uint, pluginID string, req *GetPluginRequest) (*GetPluginResponse, error)
	GetPluginInstallationStatus(ctx context.Context, orgID uint, req *GetPluginInstallationStatusRequest) (*PluginInstallationStatusResponse, error)
	ListPluginVersions(ctx context.Context, orgID uint, pluginID string) (*ListPluginVersionsResponse, error)
	DeletePlugin(ctx context.Context, orgID, uin uint, pluginID string, req *DeletePluginRequest) (*DeletePluginResponse, error)
	AddSkillPlugin(ctx context.Context, orgID, uin uint, req *AddSkillPluginRequest) error
	ResolveSkillDownloadURLs(ctx context.Context, orgID uint, req *ResolveSkillDownloadURLsRequest) (*ResolveSkillDownloadURLsResponse, error)
	ListBuiltinSkills(ctx context.Context) (*ListPluginsResponse, error)
}
