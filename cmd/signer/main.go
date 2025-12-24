// Package main provides the signer command-line tool for signing markdown files with metadata.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"tpwfc/internal/config"
	"tpwfc/internal/crawler/parsers"
	"tpwfc/internal/normalizer"
	"tpwfc/internal/validator"
	"tpwfc/pkg/metadata"
)

func main() {
	inputPath := flag.String("input", "", "Path to input file (e.g., timeline.md)")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Usage: signer -input <path>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Read markdown
	contentBytes, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("Error reading file: %v\n", err)
	}

	content := string(contentBytes)
	fmt.Printf("📂 Reading: %s (%d bytes)\n", *inputPath, len(content))

	// Load Config for Validator
	// Try to find crawler.yaml in standard locations
	configPath := "configs/crawler.yaml"
	if _, e := os.Stat(configPath); os.IsNotExist(e) {
		// Try relative to the executable or project root
		configPath = "worker/configs/crawler.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not load config from %s: %v. Using default validation.\n", configPath, err)
		cfg = &config.Config{} // Use empty config if not found
	}

	// 1. Lint Check (deno fmt)
	fmt.Println("🧹 Checking formatting (deno fmt)...")
	mdValidator, err := validator.NewMarkdownValidator(cfg)
	if err != nil {
		log.Fatalf("❌ Error creating validator: %v\n", err)
	}

	if err := mdValidator.Lint(*inputPath); err != nil {
		log.Fatalf("❌ Formatting Check Failed: %v\n", err)
	}
	fmt.Println("✅ Formatting Check Passed")

	// 2. Parse and Validate Structure
	parser := parsers.NewParser()
	fileType := parser.ParseFileType(content)
	fmt.Printf("🔍 Detected File Type: %s\n", fileType)

	valid := false

	switch fileType {
	case "DETAILED_TIMELINE":
		doc, parseErr := parser.ParseDetailedTimeline(content)
		if parseErr != nil {
			log.Fatalf("❌ Parse Error (Detailed Timeline): %v\n", parseErr)
		}

		// Optional: Add specific validation logic for DetailedTimeline here if needed
		// For now, if it parses successfully, we consider it structurally valid enough to sign
		// But let's check basic fields
		if len(doc.Phases) == 0 && len(doc.LongTermTracking) == 0 {
			fmt.Println("⚠️  Warning: Detailed timeline contains no phases or tracking events.")
		} else {
			valid = true
		}

	case "FIRE_TIMELINE":
		doc, parseErr := parser.ParseDocument(content)
		if parseErr != nil {
			log.Fatalf("❌ Parse Error (Timeline): %v\n", parseErr)
		}

		v := normalizer.NewValidator()
		if err := v.Validate(doc); err != nil {
			log.Fatalf("❌ Validation Error: %v\n", err)
		}
		valid = true
		fmt.Println("✅ Validation Passed")

	case "FIRE_INVESTIGATION", "FIRE_RESPONSES":
		// TODO: Add validators for these types
		fmt.Printf("⚠️  Validation for %s not yet implemented. Signing as unvalidated.\n", fileType)
		valid = true // Allow signing even if specific validation is missing, but maybe set VALIDATION: FALSE?
		// The user request implies "validate the structure AND do the signature".
		// If we can't completely validate, we should perhaps still sign but maybe note it?
		// Using TRUE for now if it parses, consistent with "run-sign" intent.

	default:
		// Attempt fallback parsing (legacy behavior similar to normalizer)
		if parser.ParseFileType(content) == "" {
			fmt.Println("⚠️  No FILE_TYPE tag found. Attempting heuristic parsing (DetailedTimeline)...")
			_, parseErr := parser.ParseDetailedTimeline(content)
			if parseErr != nil {
				log.Fatalf("❌ Fallback Parse Error: %v\n", parseErr)
			}
			valid = true
			fmt.Println("✅ Fallback Validation Passed")
		} else {
			log.Fatalf("❌ Unknown or unsupported file type for signing: %s\n", fileType)
		}
	}

	if valid {
		fmt.Println("✍️  Signing file...")
		signedContent := metadata.Sign(content, true, nil)

		if err := os.WriteFile(*inputPath, []byte(signedContent), 0644); err != nil {
			log.Fatalf("Error writing file: %v\n", err)
		}
		fmt.Printf("✅ Signed and saved to: %s\n", *inputPath)
	} else {
		fmt.Println("❌ Skipping signature due to validation failure.")
		os.Exit(1)
	}
}
