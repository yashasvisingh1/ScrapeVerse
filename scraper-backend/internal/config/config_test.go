package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	oldToken := os.Getenv("BRIGHT_DATA_API_TOKEN")
	oldCollector := os.Getenv("BRIGHT_DATA_COLLECTOR_ID")
	oldBaseURL := os.Getenv("BRIGHT_DATA_BASE_URL")
	oldPollInterval := os.Getenv("POLL_INTERVAL_SECONDS")
	oldPollTimeout := os.Getenv("POLL_TIMEOUT_SECONDS")
	oldOutputDir := os.Getenv("OUTPUT_DIR")
	defer func() {
		_ = os.Setenv("BRIGHT_DATA_API_TOKEN", oldToken)
		_ = os.Setenv("BRIGHT_DATA_COLLECTOR_ID", oldCollector)
		_ = os.Setenv("BRIGHT_DATA_BASE_URL", oldBaseURL)
		_ = os.Setenv("POLL_INTERVAL_SECONDS", oldPollInterval)
		_ = os.Setenv("POLL_TIMEOUT_SECONDS", oldPollTimeout)
		_ = os.Setenv("OUTPUT_DIR", oldOutputDir)
	}()

	_ = os.Setenv("BRIGHT_DATA_API_TOKEN", "token")
	_ = os.Setenv("BRIGHT_DATA_COLLECTOR_ID", "collector")
	_ = os.Setenv("BRIGHT_DATA_BASE_URL", "https://api.example.com")
	_ = os.Setenv("POLL_INTERVAL_SECONDS", "3")
	_ = os.Setenv("POLL_TIMEOUT_SECONDS", "20")
	_ = os.Setenv("OUTPUT_DIR", "./tmp-output")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BrightDataAPIToken != "token" {
		t.Fatalf("APIToken mismatch: got %q", cfg.BrightDataAPIToken)
	}
	if cfg.BrightDataCollectorID != "collector" {
		t.Fatalf("CollectorID mismatch: got %q", cfg.BrightDataCollectorID)
	}
	if cfg.BrightDataBaseURL != "https://api.example.com" {
		t.Fatalf("BaseURL mismatch: got %q", cfg.BrightDataBaseURL)
	}
	if cfg.PollInterval != 3*time.Second {
		t.Fatalf("PollInterval mismatch: got %s", cfg.PollInterval)
	}
	if cfg.PollTimeout != 20*time.Second {
		t.Fatalf("PollTimeout mismatch: got %s", cfg.PollTimeout)
	}
	if cfg.OutputDir != "./tmp-output" {
		t.Fatalf("OutputDir mismatch: got %q", cfg.OutputDir)
	}
}

func TestLoadConfigRequiresRequiredValues(t *testing.T) {
	oldToken := os.Getenv("BRIGHT_DATA_API_TOKEN")
	oldCollector := os.Getenv("BRIGHT_DATA_COLLECTOR_ID")
	defer func() {
		_ = os.Setenv("BRIGHT_DATA_API_TOKEN", oldToken)
		_ = os.Setenv("BRIGHT_DATA_COLLECTOR_ID", oldCollector)
	}()

	_ = os.Unsetenv("BRIGHT_DATA_API_TOKEN")
	_ = os.Setenv("BRIGHT_DATA_COLLECTOR_ID", "collector")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when BRIGHT_DATA_API_TOKEN is missing")
	}
}

func TestLoadUsesDotEnvWhenPresent(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	content := "BRIGHT_DATA_API_TOKEN=from-dotenv\nBRIGHT_DATA_COLLECTOR_ID=collector-from-dotenv\n"
	if err := os.WriteFile(".env", []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	oldToken := os.Getenv("BRIGHT_DATA_API_TOKEN")
	oldCollector := os.Getenv("BRIGHT_DATA_COLLECTOR_ID")
	defer func() {
		if oldToken == "" {
			_ = os.Unsetenv("BRIGHT_DATA_API_TOKEN")
		} else {
			_ = os.Setenv("BRIGHT_DATA_API_TOKEN", oldToken)
		}
		if oldCollector == "" {
			_ = os.Unsetenv("BRIGHT_DATA_COLLECTOR_ID")
		} else {
			_ = os.Setenv("BRIGHT_DATA_COLLECTOR_ID", oldCollector)
		}
	}()
	_ = os.Unsetenv("BRIGHT_DATA_API_TOKEN")
	_ = os.Unsetenv("BRIGHT_DATA_COLLECTOR_ID")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error with .env present: %v", err)
	}
	if cfg.BrightDataAPIToken != "from-dotenv" {
		t.Fatalf("APIToken from .env mismatch: got %q", cfg.BrightDataAPIToken)
	}
	if cfg.BrightDataCollectorID != "collector-from-dotenv" {
		t.Fatalf("CollectorID from .env mismatch: got %q", cfg.BrightDataCollectorID)
	}
}
