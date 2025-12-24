// Package main provides the uploader command-line tool for syncing data to Payload CMS.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"tpwfc/internal/logger"
	"tpwfc/internal/payload"
)

func main() {
	// Command line flags
	inputFile := flag.String("input", "", "Path to timeline JSON file (required)")
	endpoint := flag.String("endpoint", "http://localhost:3000/api/graphql", "GraphQL endpoint URL")
	apiKey := flag.String("api-key", "", "API key for authentication (optional)")
	email := flag.String("email", os.Getenv("ADMIN_EMAIL"), "Admin email for authentication")
	password := flag.String("password", os.Getenv("ADMIN_PASSWORD"), "Admin password for authentication")

	// Detailed mode flags
	incidentIDInt := flag.Int("incident-id", 0, "Fire incident ID (integer) for detailed upload")

	// Common flags
	language := flag.String("language", "zh-hk", "Language code")
	mode := flag.String("mode", "standard", "Upload mode: 'standard' or 'detailed'")

	// Statistics flags (passed from external scripts)
	pagesCreated := flag.Int("pages-created", -1, "Number of pages created (for stats only)")
	pagesUpdated := flag.Int("pages-updated", 0, "Number of pages updated (for stats only)")

	flag.Parse()

	// Validate required flags
	if *inputFile == "" {
		fmt.Println("Error: --input flag is required")
		fmt.Println("Usage: uploader --input <path> [options]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Initialize logger
	log := logger.NewLogger("info")
	log.Info(fmt.Sprintf("Starting uploader (%s mode): input=%s, endpoint=%s", *mode, *inputFile, *endpoint))

	// Create uploader
	uploader := payload.NewUploader(*endpoint, *apiKey, log)

	// Authenticate
	if *email != "" && *password != "" {
		log.Info("Attempting to authenticate...")

		if err := uploader.Authenticate(*email, *password); err != nil {
			log.Warn(fmt.Sprintf("Authentication failed (continuing): %v", err))
		} else {
			log.Info("✓ Authenticated successfully")
		}
	}

	if *mode == "detailed" {
		handleDetailedUpload(uploader, log, *inputFile, *incidentIDInt, *language, *pagesCreated, *pagesUpdated)
	} else {
		handleStandardUpload(uploader, log, *inputFile, *language)
	}
}

func handleStandardUpload(uploader *payload.Uploader, log *logger.Logger, inputFile, language string) {
	// Load timeline data
	data, err := payload.LoadTimelineJSON(inputFile)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to load timeline JSON: %v", err))
		os.Exit(1)
	}

	log.Info(fmt.Sprintf("Loaded timeline data: events=%d", len(data.Events)))

	// Validate required fields from JSON
	if data.BasicInfo.IncidentID == "" {
		log.Error("Error: basicInfo.incidentId is required in timeline JSON")
		os.Exit(1)
	}
	if data.BasicInfo.IncidentName == "" {
		log.Error("Error: basicInfo.incidentName is required in timeline JSON")
		os.Exit(1)
	}

	log.Info(fmt.Sprintf("Using incident from JSON: id=%s, name=%s", data.BasicInfo.IncidentID, data.BasicInfo.IncidentName))
	if data.BasicInfo.Map.Name != "" {
		log.Info(fmt.Sprintf("Map info: name=%s, url=%s", data.BasicInfo.Map.Name, data.BasicInfo.Map.URL))
	}

	result, err := uploader.Upload(data, language)
	if err != nil {
		log.Error(fmt.Sprintf("Upload failed: %v", err))
		os.Exit(1)
	}

	// Report results
	log.Info(fmt.Sprintf("Upload complete: incidentId=%d, created=%d, updated=%d, errors=%d",
		result.IncidentID, result.EventsCreated, result.EventsUpdated, len(result.Errors)))

	if len(result.Errors) > 0 {
		log.Warn(fmt.Sprintf("Some events failed to upload: count=%d", len(result.Errors)))
		os.Exit(1)
	}

	fmt.Printf("\n✓ Successfully uploaded %d events to fire incident %d\n",
		result.EventsCreated+result.EventsUpdated, result.IncidentID)
}

func handleDetailedUpload(uploader *payload.Uploader, log *logger.Logger, inputFile string, incidentID int, language string, pagesCreated int, pagesUpdated int) {
	if incidentID == 0 {
		log.Error("Error: --incident-id (integer) is required for detailed mode")
		os.Exit(1)
	}

	// Read JSON file
	jsonData, err := os.ReadFile(inputFile)
	if err != nil {
		log.Error(fmt.Sprintf("Error reading file: %v", err))
		os.Exit(1)
	}

	// Parse JSON into DetailedTimelineData structure
	var data payload.DetailedTimelineData
	if unmarshalErr := json.Unmarshal(jsonData, &data); unmarshalErr != nil {
		log.Error(fmt.Sprintf("Error parsing JSON: %v", unmarshalErr))
		os.Exit(1)
	}

	fmt.Printf("📊 Loaded: %d phases, %d long-term tracking events\n",
		len(data.Phases), len(data.LongTermTracking))

	// Upload detailed timeline data
	log.Info("Uploading detailed timeline data...")

	result, err := uploader.UploadDetailedTimeline(&data, incidentID, language)
	if err != nil {
		log.Error(fmt.Sprintf("Upload failed: %v", err))
		os.Exit(1)
	}

	fmt.Printf("\n✓ Upload complete!\n")
	fmt.Printf("   Phases created: %d, updated: %d\n", result.PhasesCreated, result.PhasesUpdated)
	fmt.Printf("   Events created: %d, updated: %d\n", result.EventsCreated, result.EventsUpdated)
	fmt.Printf("   Tracking created: %d, updated: %d\n", result.TrackingCreated, result.TrackingUpdated)
	if pagesCreated >= 0 {
		fmt.Printf("   Pages created: %d, updated: %d\n", pagesCreated, pagesUpdated)
	}

	if len(result.Errors) > 0 {
		fmt.Printf("   Errors: %d\n", len(result.Errors))

		for _, err := range result.Errors {
			fmt.Printf("     - %v\n", err)
		}
	}
}
