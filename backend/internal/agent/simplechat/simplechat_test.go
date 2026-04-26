package simplechat_test

import (
	"context"
	"os"
	"testing"

	"github.com/insmtx/SingerOS/backend/internal/agent/simplechat"
)

func TestLoadFromEnv(t *testing.T) {
	cfg := simplechat.LoadFromEnv()
	if cfg == nil {
		t.Fatal("expected config to be non-nil")
	}
	if cfg.LLMProvider != "openai" {
		t.Errorf("expected provider openai, got %s", cfg.LLMProvider)
	}
}

func TestNewRunner(t *testing.T) {
	cfg := &simplechat.Config{
		LLMProvider: "openai",
		APIKey:      "test-key",
		Model:       "gpt-4",
	}

	ctx := context.Background()
	runner, err := simplechat.NewRunner(ctx, cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if runner == nil {
		t.Fatal("expected runner to be non-nil")
	}
}

func TestRunner_Ask_RequiresAPIKey(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("skipping test: OPENAI_API_KEY not set")
	}

	cfg := &simplechat.Config{
		LLMProvider: "openai",
		APIKey:      os.Getenv("OPENAI_API_KEY"),
		Model:       "gpt-4",
	}

	ctx := context.Background()
	runner, err := simplechat.NewRunner(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	result, err := runner.Ask(ctx, "Hello, how are you?")
	if err != nil {
		t.Fatalf("failed to ask question: %v", err)
	}

	if result == nil {
		t.Fatal("expected result to be non-nil")
	}

	if result.Message == "" {
		t.Error("expected non-empty message")
	}
}

func ExampleRunner_Ask() {
	cfg := &simplechat.Config{
		LLMProvider: "openai",
		APIKey:      "your-api-key-here",
		Model:       "gpt-4",
	}

	ctx := context.Background()
	runner, err := simplechat.NewRunner(ctx, cfg)
	if err != nil {
		panic(err)
	}

	result, err := runner.Ask(ctx, "What is Go language?")
	if err != nil {
		panic(err)
	}

	println(result.Message)
}
