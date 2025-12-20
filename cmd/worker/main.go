// Package main provides the unified worker command that combines crawling, normalizing, and uploading.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"tpwfc/internal/crawler"
	"tpwfc/internal/crawler/parsers"
	"tpwfc/internal/logger"
	"tpwfc/internal/models"
	"tpwfc/internal/normalizer"
	"tpwfc/internal/payload"
)

func main() {
	// 1. Define Command-Line Flags
	// ---------------------------
	crawlerURL := flag.String("crawler-url", "", "Target Markdown URL to crawl")
	payloadURL := flag.String("payload-url", "http://localhost:3000/api/graphql", "Payload CMS GraphQL endpoint")
	apiKey := flag.String("api-key", "", "API key for authentication (optional)")
	email := flag.String("email", os.Getenv("ADMIN_EMAIL"), "Admin email for authentication")
	password := flag.String("password", os.Getenv("ADMIN_PASSWORD"), "Admin password for authentication")

	// Metadata overrides
	language := flag.String("language", "zh-hk", "Language code (zh-hk, zh-cn, en)")

	flag.Parse()

	// Initialize Logger
	log := logger.NewLogger("info")

	// Validate Inputs
	if *crawlerURL == "" {
		log.Error("Please provide a crawler URL with -crawler-url flag")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Info("🚀 Starting TPWFC Worker Pipeline")
	log.Info(fmt.Sprintf("📍 Source: %s", *crawlerURL))
	log.Info(fmt.Sprintf("🎯 Target: %s", *payloadURL))

	// 2. Ingestion (Crawler)
	// ----------------------
	log.Info("Phase 1: Ingestion (Crawling)...")

	startTime := time.Now()

	scraper := crawler.NewScraper()
	parser := parsers.NewParser()

	// Fetch raw content
	markdown, err := scraper.Scrape(*crawlerURL)
	if err != nil {
		log.Error(fmt.Sprintf("❌ Crawl failed: %v", err))
		os.Exit(1)
	}

	log.Info(fmt.Sprintf("✅ Fetched %d bytes in %v", len(markdown), time.Since(startTime)))

	// 3. Processing (Normalization)
	// -----------------------------
	log.Info("Phase 2: Processing (Parsing & Normalization)...")

	processStart := time.Now()

	doc, err := parser.ParseDocument(markdown)
	if err != nil {
		log.Error(fmt.Sprintf("❌ Parsing failed: %v", err))
		os.Exit(1)
	}

	// Check for Incident ID in document
	if doc.BasicInfo.IncidentID == "" {
		log.Error("❌ No Incident ID found in document (basicInfo.incidentId required)")
		os.Exit(1)
	}
	log.Info(fmt.Sprintf("ℹ️  Incident ID: %s", doc.BasicInfo.IncidentID))

	// Check for Incident Name in document
	if doc.BasicInfo.IncidentName == "" {
		log.Warn("⚠️  No Incident Name found in document, using incidentId as fallback")
	}
	log.Info(fmt.Sprintf("ℹ️  Incident Name: %s", doc.BasicInfo.IncidentName))

	// Normalization using Processor
	processor := normalizer.NewProcessor()

	normalizedData, err := processor.Process(doc)
	if err != nil {
		log.Error(fmt.Sprintf("❌ Normalization failed: %v", err))
		os.Exit(1)
	}

	timeline, ok := normalizedData.(*models.Timeline)
	if !ok {
		log.Error("❌ Normalization returned unexpected type")
		os.Exit(1)
	}

	log.Info(fmt.Sprintf("✅ Parsed %d events, stats, and metadata in %v", len(timeline.Events), time.Since(processStart)))

	// 4. Synchronization (Uploader)
	// -----------------------------
	log.Info("Phase 3: Synchronization (Uploading)...")

	uploader := payload.NewUploader(*payloadURL, *apiKey, log)

	// Authenticate
	if *email != "" && *password != "" {
		log.Info("🔐 Authenticating...")

		if authErr := uploader.Authenticate(*email, *password); authErr != nil {
			log.Warn(fmt.Sprintf("⚠️  Authentication failed: %v (Attempting upload anyway...)", authErr))
		} else {
			log.Info("✅ Authenticated successfully")
		}
	}

	// Upload
	result, err := uploader.Upload(timeline, *language)
	if err != nil {
		log.Error(fmt.Sprintf("❌ Upload failed: %v", err))
		os.Exit(1)
	}

	// 5. Final Report
	// ?---------------
	log.Info("✨ Pipeline Complete!")
	fmt.Println("\n------------------------------------------------")
	fmt.Printf("📊 Summary Report\n")
	fmt.Println("------------------------------------------------")
	fmt.Printf("Incident ID: %d (%s)\n", result.IncidentID, doc.BasicInfo.IncidentID)
	fmt.Printf("Events Created: %d\n", result.EventsCreated)
	fmt.Printf("Events Updated: %d\n", result.EventsUpdated)
	fmt.Printf("Total Duration: %v\n", time.Since(startTime))

	if len(result.Errors) > 0 {
		fmt.Printf("⚠️  Errors encountered: %d\n", len(result.Errors))

		for _, e := range result.Errors {
			fmt.Printf("  - %v\n", e)
		}
	}

	fmt.Println("------------------------------------------------")
}
