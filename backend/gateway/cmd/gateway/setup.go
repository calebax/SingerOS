package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insmtx/Leros/backend/gateway/adapters/qqbot"
	"github.com/insmtx/Leros/backend/gateway/pkg/config"
	"github.com/insmtx/Leros/backend/gateway/pkg/onboard"
	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive platform credential setup",
	Long: `Interactive wizard for binding platform credentials (QQ Bot, Feishu, etc.).

For platforms that support QR code scan-to-configure, you will be guided
through the device-code flow. For other platforms, you can enter credentials
manually.

Credentials are persisted to ~/.singeros/credentials.env and will be
automatically loaded by the gateway on startup.`,
	RunE: runSetup,
}

var setupPlatform string

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().StringVarP(&setupPlatform, "platform", "p", "", "platform to configure (qqbot, feishu, wecom, dingtalk)")
}

func runSetup(cmd *cobra.Command, args []string) error {
	store, err := config.NewEnvCredentialStore(config.DefaultCredentialPath())
	if err != nil {
		return fmt.Errorf("create credential store: %w", err)
	}

	// If a specific platform is requested, configure only that one.
	if setupPlatform != "" {
		return configurePlatform(cmd.Context(), store, types.ChannelCode(setupPlatform))
	}

	// Otherwise, show available platforms
	fmt.Println("Available platforms:")
	fmt.Println("  qqbot     — QQ Bot (QR scan)")
	fmt.Println("  feishu    — Feishu/Lark (QR scan)")
	fmt.Println("  wecom     — WeCom/企业微信 (QR scan)")
	fmt.Println("  dingtalk  — DingTalk/钉钉 (QR scan)")
	fmt.Println()
	fmt.Println("Usage: gateway setup --platform <name>")
	fmt.Println("Example: gateway setup --platform qqbot")
	return nil
}

// configurePlatform dispatches to the correct setup flow based on platform code.
func configurePlatform(ctx context.Context, store *config.EnvCredentialStore, code types.ChannelCode) error {
	// Check if already configured
	if store.Exists(string(code)) {
		fmt.Printf("Platform %s is already configured.\n", code)
		creds, _ := store.Load(string(code))
		fmt.Printf("Existing credentials: %v\n", maskCredentials(creds))
		fmt.Print("Reconfigure? (y/N): ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			return nil
		}
	}

	switch code {
	case "qqbot":
		return setupQQBot(ctx, store)
	case "feishu":
		return setupFeishu(ctx, store)
	case "wecom":
		return setupWeCom(ctx, store)
	case "dingtalk":
		return setupDingTalk(ctx, store)
	default:
		return fmt.Errorf("unknown platform: %s (available: qqbot, feishu, wecom, dingtalk)", code)
	}
}

// setupQQBot runs the QQ Bot QR scan-to-configure flow.
func setupQQBot(ctx context.Context, store *config.EnvCredentialStore) error {
	fmt.Println()
	fmt.Println("=== QQ Bot Setup ===")
	fmt.Println()
	fmt.Println("This will guide you through binding your QQ Bot credentials.")
	fmt.Println("You will need to scan a QR code with QQ on your phone.")
	fmt.Println()

	// Option: QR scan or manual input
	fmt.Print("Use QR code scan? (Y/n): ")
	var answer string
	fmt.Scanln(&answer)

	if answer == "n" || answer == "N" {
		return setupQQBotManual(store)
	}

	// QR flow
	onboarder := qqbot.NewOnboarder()
	engine := onboard.NewEngine(onboarder, store)

	fmt.Println()
	fmt.Println("Generating QR code...")
	fmt.Println("Open QQ on your phone and scan the QR code below.")
	fmt.Println("(If you can't see the QR code, open the URL manually.)")

	result, err := engine.Run(ctx)
	if err != nil {
		return fmt.Errorf("QQ Bot onboarding failed: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ QQ Bot configured successfully!")
	fmt.Printf("  App ID: %s\n", result.Credentials["app_id"])
	if result.UserOpenID != "" {
		fmt.Printf("  Scanner OpenID: %s\n", result.UserOpenID)
	}
	fmt.Printf("  Credentials saved to: %s\n", config.DefaultCredentialPath())
	return nil
}

// setupQQBotManual prompts for manual credential entry.
func setupQQBotManual(store *config.EnvCredentialStore) error {
	fmt.Println()
	fmt.Println("Manual QQ Bot credential entry.")
	fmt.Println("You can find these in the QQ Open Platform console: https://q.qq.com")

	var appID, clientSecret string
	fmt.Print("App ID: ")
	fmt.Scanln(&appID)
	fmt.Print("Client Secret: ")
	fmt.Scanln(&clientSecret)

	if appID == "" || clientSecret == "" {
		return fmt.Errorf("app_id and client_secret are required")
	}

	return store.Save("qqbot", map[string]string{
		"app_id":        appID,
		"client_secret": clientSecret,
	})
}

// setupFeishu runs the Feishu/Lark QR scan-to-configure flow (placeholder).
func setupFeishu(ctx context.Context, store *config.EnvCredentialStore) error {
	fmt.Println()
	fmt.Println("=== Feishu Setup ===")
	fmt.Println("Feishu onboard is not yet implemented. Please configure credentials manually.")
	return setupStandardPlatform(store, "feishu", map[string]string{
		"app_id":            "Feishu App ID",
		"app_secret":        "Feishu App Secret",
		"verification_token": "Verification Token (optional)",
	})
}

// setupWeCom runs the WeCom bot setup flow (placeholder).
func setupWeCom(ctx context.Context, store *config.EnvCredentialStore) error {
	fmt.Println()
	fmt.Println("=== WeCom Setup ===")
	fmt.Println("WeCom onboard is not yet implemented. Please configure credentials manually.")
	return setupStandardPlatform(store, "wecom", map[string]string{
		"corp_id":         "Corp ID",
		"agent_id":        "Agent ID",
		"secret":          "Agent Secret",
		"token":           "Callback Token",
		"encoding_aes_key": "Encoding AES Key",
	})
}

// setupDingTalk runs the DingTalk device code flow (placeholder).
func setupDingTalk(ctx context.Context, store *config.EnvCredentialStore) error {
	fmt.Println()
	fmt.Println("=== DingTalk Setup ===")
	fmt.Println("DingTalk onboard is not yet implemented. Please configure credentials manually.")
	return setupStandardPlatform(store, "dingtalk", map[string]string{
		"client_id":     "Client ID (AppKey)",
		"client_secret": "Client Secret (AppSecret)",
	})
}

// setupStandardPlatform provides a generic interactive credential entry for any platform.
//
// This mirrors Hermes' _setup_standard_platform() — it shows the platform name,
// lists the required fields with descriptions, and prompts the user for each one.
func setupStandardPlatform(store *config.EnvCredentialStore, platform string, fields map[string]string) error {
	fmt.Println()
	fmt.Println("Manual credential entry.")
	fmt.Println("Required fields:")

	creds := make(map[string]string)
	for key, desc := range fields {
		fmt.Printf("  %s (%s): ", key, desc)
		var value string
		fmt.Scanln(&value)
		if value != "" {
			creds[key] = value
		}
	}

	if len(creds) == 0 {
		return fmt.Errorf("no credentials entered")
	}

	return store.Save(platform, creds)
}

// maskCredentials returns a copy with secret values masked.
func maskCredentials(creds map[string]string) map[string]string {
	masked := make(map[string]string)
	for k, v := range creds {
		if len(v) > 8 {
			masked[k] = v[:4] + "****" + v[len(v)-4:]
		} else {
			masked[k] = "****"
		}
	}
	return masked
}
