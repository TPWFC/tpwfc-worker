// Package main provides the seed command-line tool for post-deploy data upload.
// It waits for the web service to be healthy, then runs crawler and uploads data.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
)

// Config holds the seeder configuration.
type Config struct {
	WebURL          string
	GraphQLEndpoint string
	HealthTimeout   time.Duration
	AdminEmail      string
	AdminPassword   string
	SigningSecret   string
	BinDir          string
	DataDir         string
	ConfigPath      string
}

func logInfo(msg string) {
	fmt.Printf("%s[SEEDER]%s %s\n", colorGreen, colorReset, msg)
}

func logWarn(msg string) {
	fmt.Printf("%s[SEEDER]%s %s\n", colorYellow, colorReset, msg)
}

func logError(msg string) {
	fmt.Printf("%s[SEEDER]%s %s\n", colorRed, colorReset, msg)
}

func main() {
	// Parse configuration from flags and environment
	cfg := parseConfig()

	// Wait for web service
	if !waitForWeb(cfg) {
		logError("Aborting seeding - web service not available")
		os.Exit(1)
	}

	// Check required environment variables
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		logError("ADMIN_EMAIL and ADMIN_PASSWORD must be set")
		os.Exit(1)
	}

	if cfg.SigningSecret == "" {
		logError("SEED_SIGNING_SECRET must be set")
		os.Exit(1)
	}

	// Run formatter on source files
	logInfo("Formatting source markdown files...")
	runFormatter(cfg)

	// Run crawler
	logInfo("Running crawler...")
	if err := runCrawler(cfg); err != nil {
		logError(fmt.Sprintf("Crawler failed: %v", err))
		os.Exit(1)
	}

	// Upload timelines for each language
	uploadTimelines(cfg)

	logInfo("===========================================")
	logInfo("Seeding complete!")
	logInfo("===========================================")
}

func parseConfig() Config {
	webURL := flag.String("web-url", "", "Web service URL (default: NEXT_PUBLIC_BASE_URL or http://tpwfc-web:3000)")
	healthTimeout := flag.Duration("health-timeout", 120*time.Second, "Health check timeout")
	binDir := flag.String("bin-dir", "./bin", "Directory containing binaries")
	dataDir := flag.String("data-dir", "./data", "Data directory root")
	configPath := flag.String("config", "./configs/crawler.yaml", "Crawler config path")
	flag.Parse()

	// Resolve web URL with fallback
	url := *webURL
	if url == "" {
		url = os.Getenv("NEXT_PUBLIC_BASE_URL")
	}
	if url == "" {
		url = "http://tpwfc-web:3000"
	}

	return Config{
		WebURL:          url,
		GraphQLEndpoint: url + "/api/graphql",
		HealthTimeout:   *healthTimeout,
		AdminEmail:      os.Getenv("ADMIN_EMAIL"),
		AdminPassword:   os.Getenv("ADMIN_PASSWORD"),
		SigningSecret:   os.Getenv("SEED_SIGNING_SECRET"),
		BinDir:          *binDir,
		DataDir:         *dataDir,
		ConfigPath:      *configPath,
	}
}

func waitForWeb(cfg Config) bool {
	startTime := time.Now()
	logInfo(fmt.Sprintf("Waiting for web service at %s...", cfg.WebURL))

	client := &http.Client{Timeout: 5 * time.Second}

	for {
		resp, err := client.Get(cfg.WebURL)
		if err == nil {
			statusCode := resp.StatusCode
			// Close body immediately after reading status
			if closeErr := resp.Body.Close(); closeErr != nil {
				logWarn(fmt.Sprintf("Failed to close response body: %v", closeErr))
			}
			if statusCode >= 200 && statusCode < 400 {
				logInfo(fmt.Sprintf("Web service is ready! (HTTP %d)", statusCode))
				// Wait for database schema initialization (Payload push: true)
				logInfo("Waiting for database schema initialization...")
				time.Sleep(15 * time.Second)

				// Verify GraphQL is actually ready by testing introspection
				if waitForGraphQL(cfg, client) {
					return true
				}
				logWarn("GraphQL not ready after initial wait, continuing to retry...")
			}
		}

		elapsed := time.Since(startTime)
		if elapsed >= cfg.HealthTimeout {
			logError(fmt.Sprintf("Web service failed to start within %v", cfg.HealthTimeout))
			return false
		}

		fmt.Print(".")
		time.Sleep(2 * time.Second)
	}
}

// waitForGraphQL verifies the GraphQL endpoint is responding with valid schema
func waitForGraphQL(cfg Config, client *http.Client) bool {
	// Simple introspection query to verify schema is loaded
	query := `{"query": "{ __typename }"}`

	for i := 0; i < 5; i++ {
		req, err := http.NewRequest("POST", cfg.GraphQLEndpoint, strings.NewReader(query))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check if we got a valid GraphQL response (not an error about missing tables)
		if resp.StatusCode == 200 && !strings.Contains(string(body), "Failed query") {
			logInfo("GraphQL endpoint is ready")
			return true
		}

		logWarn(fmt.Sprintf("GraphQL not ready (attempt %d/5), waiting...", i+1))
		time.Sleep(3 * time.Second)
	}

	return false
}

func runFormatter(cfg Config) {
	formatterPath := filepath.Join(cfg.BinDir, "formatter")
	sourcePath := filepath.Join(cfg.DataDir, "source")

	cmd := exec.Command(formatterPath, "-path", sourcePath, "-write")
	// Ignore errors - matches original script behavior
	_ = cmd.Run()
}

func runCrawler(cfg Config) error {
	crawlerPath := filepath.Join(cfg.BinDir, "crawler")

	cmd := exec.Command(crawlerPath, "-config", cfg.ConfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func uploadTimelines(cfg Config) {
	languages := []struct {
		code    string
		dirName string
	}{
		{code: "zh-hk", dirName: "zh-hk"},
		{code: "en-us", dirName: "en-us"},
		{code: "zh-cn", dirName: "zh-cn"},
	}

	for _, lang := range languages {
		jsonPath := filepath.Join(cfg.DataDir, "fire", "WANG_FUK_COURT_FIRE_2025", lang.dirName, "timeline.json")

		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			logWarn(fmt.Sprintf("Timeline not found for %s, skipping: %s", lang.code, jsonPath))
			continue
		}

		logInfo(fmt.Sprintf("Uploading %s timeline...", lang.code))

		if err := runUploader(cfg, jsonPath, lang.code); err != nil {
			logError(fmt.Sprintf("Failed to upload %s timeline: %v", lang.code, err))
		}
	}
}

func runUploader(cfg Config, inputPath, language string) error {
	uploaderPath := filepath.Join(cfg.BinDir, "uploader")

	args := []string{
		"--input", inputPath,
		"--endpoint", cfg.GraphQLEndpoint,
		"--email", cfg.AdminEmail,
		"--password", cfg.AdminPassword,
		"--signing-secret", cfg.SigningSecret,
		"--language", language,
	}

	cmd := exec.Command(uploaderPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Use Run() instead of CombinedOutput() since we already set Stdout/Stderr
	return cmd.Run()
}
