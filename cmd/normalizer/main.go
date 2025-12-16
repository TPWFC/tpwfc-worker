// Package main provides the normalizer command-line tool for transforming crawled data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"tpwfc/internal/crawler/parsers"
)

func main() {
	inputPath := flag.String("input", "", "Path to input file (e.g., detailed_timeline.md)")
	outputPath := flag.String("output", "", "Path to output JSON file")
	flag.Parse()

	if *inputPath == "" || *outputPath == "" {
		fmt.Println("Usage: normalizer -input <input.md> -output <output.json>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Read markdown
	content, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("Error reading file: %v\n", err)
	}

	fmt.Printf("📂 Reading: %s (%d bytes)\n", *inputPath, len(content))

	// Parse based on file type
	parser := parsers.NewParser()
	fileType := parser.ParseFileType(string(content))
	fmt.Printf("🔍 Detected File Type: %s\n", fileType)

	var output interface{}

	switch fileType {
	case "DETAILED_TIMELINE":
		doc, parseErr := parser.ParseDetailedTimeline(string(content))
		if parseErr != nil {
			log.Fatalf("Error parsing detailed timeline: %v\n", parseErr)
		}
		fmt.Printf("📊 Parsed: %d phases, %d long-term tracking events, %d category metrics, %d notes\n",
			len(doc.Phases), len(doc.LongTermTracking), len(doc.CategoryMetrics), len(doc.Notes))

		// Map to map[string]interface{} for consistency with previous output structure
		output = map[string]interface{}{
			"phases":           doc.Phases,
			"longTermTracking": doc.LongTermTracking,
			"categoryMetrics":  doc.CategoryMetrics,
			"notes":            doc.Notes,
		}

	case "FIRE_TIMELINE":
		doc, parseErr := parser.ParseDocument(string(content))
		if parseErr != nil {
			log.Fatalf("Error parsing timeline: %v\n", parseErr)
		}
		fmt.Printf("📊 Parsed standard timeline: %d events\n", len(doc.Events))
		output = doc

	case "FIRE_INVESTIGATION", "FIRE_RESPONSES":
		fmt.Printf("⚠️  Parsing for %s not yet implemented. Passing raw content in wrapper.\n", fileType)
		// Return a generic wrapper for now
		output = map[string]interface{}{
			"fileType": fileType,
			"raw":      string(content),
		}

	default:
		// Fallback detection (legacy)
		if parser.ParseFileType(string(content)) == "" {
			fmt.Println("⚠️  No FILE_TYPE found. Attempting heuristic detection...")
			// TODO: Add heuristic or default to DetailedTimeline logic as it was default before?
			// The original code assumed DetailedTimeline because it called parser.ParseDetailedTimeline directly.
			// Let's fallback to that for backward compatibility.
			doc, parseErr := parser.ParseDetailedTimeline(string(content))
			if parseErr != nil {
				log.Fatalf("Error parsing (fallback): %v\n", parseErr)
			}
			fmt.Printf("📊 Parsed (fallback): %d phases\n", len(doc.Phases))
			output = map[string]interface{}{
				"phases":           doc.Phases,
				"longTermTracking": doc.LongTermTracking,
				"notes":            doc.Notes,
			}
		} else {
			log.Fatalf("Unknown file type: %s\n", fileType)
		}
	}

	// Ensure directory exists
	if mkdirErr := os.MkdirAll(filepath.Dir(*outputPath), 0755); mkdirErr != nil {
		log.Fatalf("Error creating directory: %v\n", mkdirErr)
	}

	// Marshal and save
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v\n", err)
	}

	if err := os.WriteFile(*outputPath, jsonData, 0644); err != nil {
		log.Fatalf("Error writing file: %v\n", err)
	}

	fmt.Printf("✅ Saved to: %s\n", *outputPath)
}
