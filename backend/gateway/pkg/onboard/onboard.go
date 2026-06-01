// Package onboard provides a unified framework for platform credential binding.
//
// Many IM platforms (Feishu, DingTalk, QQ Bot, WeCom) use a "device code" or
// "QR code scan-to-configure" flow for bot registration. This package defines
// the common abstractions and a generic engine that handles the lifecycle:
//
//	init → begin → render QR → poll → decrypt → probe → save
//
// Each platform adapter implements the PlatformOnboarder interface, and the
// Engine orchestrates the user interaction flow.
package onboard

import "context"

// Status represents the current state of an onboard flow.
type Status int

const (
	StatusPending   Status = iota // waiting for user to scan QR
	StatusCompleted               // credentials obtained successfully
	StatusExpired                 // QR code expired, need to restart
	StatusFailed                  // unrecoverable error
	StatusProbing                 // verifying obtained credentials
)

// Result holds the credentials obtained from a successful onboard flow.
//
// The exact fields populated depend on the platform. Adapters interpret
// this map to build their configuration.
type Result struct {
	// Platform is the channel code (e.g., "qqbot", "feishu").
	Platform string

	// Credentials are the platform-specific key-value pairs obtained
	// from the registration flow (e.g., app_id, app_secret, bot_token).
	Credentials map[string]string

	// UserOpenID is the OpenID of the user who scanned the QR code, if available.
	UserOpenID string

	// Extra holds any additional platform-specific data.
	Extra map[string]string
}

// PlatformOnboarder is implemented by each platform adapter that supports
// QR/device-code binding. The engine calls these methods in order.
type PlatformOnboarder interface {
	// PlatformCode returns the channel identifier.
	PlatformCode() string

	// Init starts the registration flow on the platform's server.
	// Returns a state token that Begin will use.
	Init(ctx context.Context) (state any, err error)

	// Begin creates the QR code / device code for the user to scan.
	// The state is the value returned by Init.
	// Returns:
	//   - qrURL: the URL to render as QR code
	//   - manualURL: a fallback URL the user can open manually
	//   - pollToken: opaque token for the Poll method
	Begin(ctx context.Context, state any) (qrURL string, manualURL string, pollToken any, err error)

	// Poll checks the status of the registration.
	// Returns the current status and, if completed, the credentials.
	Poll(ctx context.Context, state any, pollToken any) (Status, *Result, error)

	// Decrypt processes an encrypted result (e.g., AES decryption of client_secret).
	// Not all platforms need this — return the result unchanged if encryption is not used.
	Decrypt(ctx context.Context, state any, raw *Result) (*Result, error)

	// Probe verifies that the obtained credentials are valid by making a test
	// API call (e.g., GET /bot/info). Returns an error if the credentials are invalid.
	Probe(ctx context.Context, result *Result) error
}

// CredentialStore persists and retrieves platform credentials.
type CredentialStore interface {
	// Save persists platform credentials.
	Save(platform string, credentials map[string]string) error

	// Load retrieves platform credentials.
	Load(platform string) (map[string]string, error)

	// Remove deletes platform credentials.
	Remove(platform string) error

	// Exists returns true if credentials exist for the platform.
	Exists(platform string) bool
}
