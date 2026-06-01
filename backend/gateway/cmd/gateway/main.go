// Package main is the entry point for the SingerOS message gateway process.
//
// The gateway runs as an independent process that:
//   - Loads configuration for all enabled channel adapters
//   - Manages adapter lifecycles (Connect / Disconnect)
//   - Routes inbound messages through the dispatch pipeline
//   - Routes outbound messages back to the appropriate channel sender
//
// Usage:
//
//	gateway run --config gateway.yaml
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	gatewayconfig "github.com/insmtx/Leros/backend/gateway/pkg/config"
	"github.com/insmtx/Leros/backend/gateway/pkg/core"
	"github.com/insmtx/Leros/backend/gateway/pkg/dispatch"
	"github.com/insmtx/Leros/backend/gateway/pkg/infra"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

var configPath string

var rootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "SingerOS Message Gateway",
	Long: `SingerOS Message Gateway is an independent process that manages
multi-channel messaging (IM + Webhook) for the SingerOS platform.

It receives messages from external platforms (Feishu, WeChat, GitHub, etc.),
normalizes them, and publishes them to the event bus for downstream processing.
It also routes outbound messages back to the correct channel.`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the message gateway",
	Long:  "Start the gateway process with the specified configuration file.",
	RunE:  runGateway,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&configPath, "config", "c", "gateway.yaml", "path to gateway configuration file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runGateway(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// 1. Load configuration
	cfg, err := gatewayconfig.LoadGatewayConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Load credentials from env store (e.g., ~/.singeros/credentials.env)
	store, err := gatewayconfig.NewEnvCredentialStore(gatewayconfig.DefaultCredentialPath())
	if err != nil {
		return fmt.Errorf("create credential store: %w", err)
	}

	// Merge credentials into channel configs
	for _, channelCode := range cfg.ChannelCodes() {
		if creds, err := store.Load(string(channelCode)); err == nil && len(creds) > 0 {
			chCfg := cfg.Channels[channelCode]
			if chCfg.Extra == nil {
				chCfg.Extra = make(map[string]any)
			}
			for k, v := range creds {
				chCfg.Extra[k] = v
			}
			cfg.Channels[channelCode] = chCfg
		}
	}

	// 3. Build registry with built-in adapters
	registry := core.NewRegistry()
	registerBuiltins(registry, cfg)

	// 3. Connect to NATS for event publishing
	var publisher dispatch.EventPublisher
	if cfg.NATS.URL != "" {
		np, err := infra.NewNatsPublisher(cfg.NATS.URL)
		if err != nil {
			return fmt.Errorf("connect to NATS: %w", err)
		}
		defer np.Close()
		publisher = np
		fmt.Printf("Connected to NATS at %s\n", cfg.NATS.URL)
	} else {
		fmt.Println("No NATS URL configured, running without event publishing")
	}

	// 4. Build dispatch infrastructure
	router := dispatch.NewDeliveryRouter()
	authGate := buildAuthGate(cfg)
	gw := dispatch.NewMessageGateway(router, publisher,
		dispatch.WithAuthGate(authGate),
	)

	// 5. Instantiate and configure enabled adapters
	type managedAdapter struct {
		info    types.AdapterInfo
		adapter any
	}
	var adapters []managedAdapter

	for _, entry := range registry.Enabled() {
		chCfg := channelConfig(cfg, entry.Code)
		adapter, err := entry.Factory(chCfg)
		if err != nil {
			return fmt.Errorf("create adapter %s: %w", entry.Code, err)
		}
		adapters = append(adapters, managedAdapter{info: adapter.(core.Connector).Info(), adapter: adapter})
	}

	// 7. Wire adapters to gateway and register senders
	for _, ma := range adapters {
		a := ma.adapter

		// Wire Receiver → Gateway
		if receiver, ok := a.(core.Receiver); ok {
			if err := receiver.OnMessage(gw.AdapterCallback(ma.info.Code)); err != nil {
				return fmt.Errorf("wire receiver %s: %w", ma.info.Code, err)
			}
		}

		// Wire Sender → DeliveryRouter
		if sender, ok := a.(core.Sender); ok {
			router.Register(ma.info.Code, sender)
		}

		// Register webhook routes
		if wh, ok := a.(core.WebhookReceiver); ok {
			mux := http.NewServeMux() // per-adapter mux for webhook routes
			if err := wh.RegisterWebhookRoutes(mux); err != nil {
				return fmt.Errorf("register webhook routes for %s: %w", ma.info.Code, err)
			}
			// Webhook routes are served via the gateway's HTTP server
			fmt.Printf("Webhook routes registered for %s\n", ma.info.Code)
		}
	}

	// 8. Connect all adapters
	for _, ma := range adapters {
		if lifecycle, ok := ma.adapter.(core.Lifecycle); ok {
			fmt.Printf("Connecting %s...\n", ma.info.Code)
			if err := lifecycle.Connect(ctx); err != nil {
				return fmt.Errorf("connect %s: %w", ma.info.Code, err)
			}
			fmt.Printf("%s connected\n", ma.info.Code)
		}
	}

	fmt.Printf("Gateway started with %d channel(s)\n", len(adapters))

	// 7. Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	fmt.Printf("Received %s, shutting down...\n", sig)

	// 8. Graceful shutdown
	cancel()

	for _, ma := range adapters {
		if lifecycle, ok := ma.adapter.(core.Lifecycle); ok {
			fmt.Printf("Disconnecting %s...\n", ma.info.Code)
			if err := lifecycle.Disconnect(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Error disconnecting %s: %v\n", ma.info.Code, err)
			}
			fmt.Printf("%s disconnected\n", ma.info.Code)
		}
	}

	fmt.Println("Gateway stopped")
	return nil
}

// buildAuthGate constructs the authorization gate from configuration.
func buildAuthGate(cfg *gatewayconfig.GatewayConfig) *dispatch.AuthGate {
	gate := dispatch.NewAuthGate()

	// Global rules first (checked before channel-specific, but both in rule chain)
	if len(cfg.Auth.GlobalDenylist) > 0 {
		gate.AddRule(dispatch.DenylistUsers(cfg.Auth.GlobalDenylist...))
	}
	if len(cfg.Auth.GlobalAllowlist) > 0 {
		gate.AddRule(dispatch.AllowlistUsers(cfg.Auth.GlobalAllowlist...))
	}

	return gate
}

// channelConfig converts the gateway-level channel config to the adapter's config type.
func channelConfig(cfg *gatewayconfig.GatewayConfig, code types.ChannelCode) types.ChannelConfig {
	chCfg, ok := cfg.Channels[code]
	if !ok {
		return types.ChannelConfig{Code: code}
	}
	return types.ChannelConfig{
		Code:     code,
		Disabled: !chCfg.Enabled,
		Extra:    chCfg.Extra,
	}
}

// registerBuiltins registers all built-in channel adapters.
// Plugin adapters should register via their init() functions before this runs.
func registerBuiltins(registry *core.Registry, cfg *gatewayconfig.GatewayConfig) {
	// GitHub webhook receiver
	registry.MustRegister(core.ChannelEntry{
		Code:        "github",
		Label:       "GitHub",
		Description: "Receives GitHub webhook events (PR, issue, push, etc.)",
		Version:     "1.0.0",
		Order:       100,
		Capabilities: types.ChannelCapabilities{
			SupportsWebhook: true,
			SupportsStream:  false,
			NeedsLongConn:   false,
			MaxMessageLen:   0,
		},
		Enabled: func() bool {
			chCfg, ok := cfg.Channels["github"]
			return ok && chCfg.Enabled
		},
		Factory: func(chCfg types.ChannelConfig) (any, error) {
			return newGitHubAdapter(chCfg), nil
		},
	})

	// QQ Bot adapter (WebSocket gateway)
	registry.MustRegister(core.ChannelEntry{
		Code:        "qqbot",
		Label:       "QQ Bot",
		Description: "QQ Bot channel via WebSocket gateway (C2C, group @, guild, DM, interactions)",
		Version:     "1.0.0",
		Order:       200,
		Capabilities: types.ChannelCapabilities{
			SupportsIM:    true,
			SupportsStream: false,
			NeedsLongConn:  true,
			MaxMessageLen:  4000,
		},
		Enabled: func() bool {
			chCfg, ok := cfg.Channels["qqbot"]
			return ok && chCfg.Enabled
		},
		Factory: func(chCfg types.ChannelConfig) (any, error) {
			return newQQBotAdapter(chCfg), nil
		},
	})

	// Feishu/Lark adapter (WebSocket + webhook dual mode)
	registry.MustRegister(core.ChannelEntry{
		Code:        "feishu",
		Label:       "Feishu/Lark",
		Description: "Feishu/Lark IM bot via WebSocket + webhook dual mode",
		Version:     "1.0.0",
		Order:       210,
		Capabilities: types.ChannelCapabilities{
			SupportsIM:      true,
			SupportsWebhook:  true,
			SupportsStream:   false,
			NeedsLongConn:    true,
			MaxMessageLen:    20000,
		},
		Enabled: func() bool {
			chCfg, ok := cfg.Channels["feishu"]
			return ok && chCfg.Enabled
		},
		Factory: func(chCfg types.ChannelConfig) (any, error) {
			return newFeishuAdapter(chCfg), nil
		},
	})
}
