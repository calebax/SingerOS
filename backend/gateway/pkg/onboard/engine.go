package onboard

import (
	"context"
	"fmt"
	"time"
)

// EngineOption configures the onboard engine.
type EngineOption func(*Engine)

// WithPollInterval sets the polling interval for checking QR scan status.
func WithPollInterval(d time.Duration) EngineOption {
	return func(e *Engine) {
		e.pollInterval = d
	}
}

// WithTimeout sets the maximum duration for the entire onboard flow.
func WithTimeout(d time.Duration) EngineOption {
	return func(e *Engine) {
		e.timeout = d
	}
}

// WithMaxRefreshes sets how many times the QR code can be refreshed on expiry.
func WithMaxRefreshes(n int) EngineOption {
	return func(e *Engine) {
		e.maxRefreshes = n
	}
}

// Engine orchestrates the platform onboarding flow.
//
// Usage:
//
//	engine := onboard.NewEngine(onboarder, store)
//	result, err := engine.Run(ctx)
//	if err != nil {
//	    // handle error
//	}
//	fmt.Printf("Platform configured: %s\n", result.Platform)
type Engine struct {
	onboarder    PlatformOnboarder
	store        CredentialStore
	pollInterval time.Duration
	timeout      time.Duration
	maxRefreshes int
}

// NewEngine creates an onboard engine for a platform.
func NewEngine(onboarder PlatformOnboarder, store CredentialStore, opts ...EngineOption) *Engine {
	e := &Engine{
		onboarder:    onboarder,
		store:        store,
		pollInterval: 2 * time.Second,
		timeout:      10 * time.Minute,
		maxRefreshes: 3,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Run executes the full onboard flow and persists credentials on success.
//
// Flow:
//
//	Init → Begin → (render QR) → Poll loop → Decrypt → Probe → Save
//
// On QR expiry, the flow restarts from Init (up to maxRefreshes times).
// On timeout, an error is returned.
func (e *Engine) Run(ctx context.Context) (*Result, error) {
	deadline := time.Now().Add(e.timeout)

	for refresh := 0; refresh <= e.maxRefreshes; refresh++ {
		// 1. Init
		state, err := e.onboarder.Init(ctx)
		if err != nil {
			return nil, fmt.Errorf("onboard init: %w", err)
		}

		// 2. Begin — get QR code
		qrURL, manualURL, pollToken, err := e.onboarder.Begin(ctx, state)
		if err != nil {
			return nil, fmt.Errorf("onboard begin: %w", err)
		}

		// 3. Render QR (caller handles display via the Renderer interface)
		e.displayQR(qrURL, manualURL)

		// 4. Poll loop
		for time.Now().Before(deadline) {
			status, result, err := e.onboarder.Poll(ctx, state, pollToken)
			if err != nil {
				// Non-fatal poll errors — retry after interval
				time.Sleep(e.pollInterval)
				continue
			}

			switch status {
			case StatusCompleted:
				return e.finish(ctx, state, result)

			case StatusExpired:
				// Break inner loop to restart from Init
				fmt.Println("  QR code expired, refreshing...")
				break

			case StatusFailed:
				return nil, fmt.Errorf("onboard failed")

			case StatusPending:
				// Continue polling
				time.Sleep(e.pollInterval)
				continue
			}

			// If we got here, status was Expired — break the poll loop
			if status == StatusExpired {
				break
			}
		}
	}

	return nil, fmt.Errorf("onboard timed out after %v (%d refreshes)", e.timeout, e.maxRefreshes)
}

// finish completes the onboard flow: decrypt → probe → save.
func (e *Engine) finish(ctx context.Context, state any, raw *Result) (*Result, error) {
	// 5. Decrypt
	result, err := e.onboarder.Decrypt(ctx, state, raw)
	if err != nil {
		return nil, fmt.Errorf("onboard decrypt: %w", err)
	}

	// 6. Probe
	fmt.Printf("  Verifying credentials for %s...\n", result.Platform)
	if err := e.onboarder.Probe(ctx, result); err != nil {
		return nil, fmt.Errorf("onboard probe: %w", err)
	}

	// 7. Save
	fmt.Printf("  Saving credentials for %s...\n", result.Platform)
	if err := e.store.Save(result.Platform, result.Credentials); err != nil {
		return nil, fmt.Errorf("onboard save: %w", err)
	}

	return result, nil
}

// displayQR outputs the QR URL and manual fallback URL.
func (e *Engine) displayQR(qrURL, manualURL string) {
	fmt.Println()
	fmt.Printf("  Scan the QR code below, or open this URL in your app:\n")
	fmt.Printf("  %s\n", manualURL)
	fmt.Printf("  (QR URL: %s)\n", qrURL)
	fmt.Println()
}
