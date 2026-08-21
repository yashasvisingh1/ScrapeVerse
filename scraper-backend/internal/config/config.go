package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config contains all runtime settings required to interact with Bright Data and manage output files.
type Config struct {
	BrightDataAPIToken    string
	BrightDataCollectorID string
	BrightDataBaseURL     string
	PollInterval          time.Duration
	PollTimeout           time.Duration
	OutputDir             string
}

func Load() (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		// Ignore missing dotenv files; users can still provide variables via environment or shell.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load .env: %w", err)
		}
	}

	apiToken := strings.TrimSpace(os.Getenv("BRIGHT_DATA_API_TOKEN"))
	collectorID := strings.TrimSpace(os.Getenv("BRIGHT_DATA_COLLECTOR_ID"))
	baseURL := strings.TrimSpace(os.Getenv("BRIGHT_DATA_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.brightdata.com"
	}

	pollIntervalSeconds := getEnvInt("POLL_INTERVAL_SECONDS", 5)
	pollTimeoutSeconds := getEnvInt("POLL_TIMEOUT_SECONDS", 300)
	outputDir := strings.TrimSpace(os.Getenv("OUTPUT_DIR"))
	if outputDir == "" {
		outputDir = "./output"
	}

	if apiToken == "" {
		return nil, fmt.Errorf("BRIGHT_DATA_API_TOKEN is required")
	}
	if collectorID == "" {
		return nil, fmt.Errorf("BRIGHT_DATA_COLLECTOR_ID is required")
	}
	if pollIntervalSeconds <= 0 {
		return nil, fmt.Errorf("POLL_INTERVAL_SECONDS must be greater than zero")
	}
	if pollTimeoutSeconds <= 0 {
		return nil, fmt.Errorf("POLL_TIMEOUT_SECONDS must be greater than zero")
	}
	if pollTimeoutSeconds < pollIntervalSeconds {
		return nil, fmt.Errorf("POLL_TIMEOUT_SECONDS must be greater than or equal to POLL_INTERVAL_SECONDS")
	}

	return &Config{
		BrightDataAPIToken:    apiToken,
		BrightDataCollectorID: collectorID,
		BrightDataBaseURL:     baseURL,
		PollInterval:          time.Duration(pollIntervalSeconds) * time.Second,
		PollTimeout:           time.Duration(pollTimeoutSeconds) * time.Second,
		OutputDir:             outputDir,
	}, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func getEnvInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
