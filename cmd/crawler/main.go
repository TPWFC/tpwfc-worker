// Package main provides the crawler command-line tool for extracting timeline data from sources.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"tpwfc/internal/config"
	"tpwfc/internal/crawler"
	"tpwfc/internal/crawler/parsers"
	"tpwfc/internal/validator"
)

func main() {
	// Define command-line flags
	configFile := flag.String("config", "", "Path to YAML configuration file")
	targetURL := flag.String("url", "", "Target markdown URL to crawl (overrides config)")
	localFile := flag.String("file", "", "Local markdown file path to parse (bypasses URL crawling)")
	output := flag.String("output", "", "Output JSON file path (overrides config)")
	format := flag.String("format", "", "Output format (overrides config)")
	showValidation := flag.Bool("validate", false, "Validate markdown format before crawling")
	showUsage := flag.Bool("help", false, "Show usage information")

	flag.Parse()

	if *showUsage {
		printUsage()
		os.Exit(0)
	}

	// If local file is provided, use local file mode
	if *localFile != "" {
		runLocalFileMode(*localFile, *output, *showValidation)

		return
	}

	var cfg *config.Config

	var err error

	// Load configuration
	if *configFile != "" {
		fmt.Printf("⚙️  Loading configuration from: %s\n", *configFile)

		cfg, err = config.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("❌ Failed to load config: %v\n", err)
		}

		fmt.Printf("✅ Configuration loaded: %s\n\n", cfg)
	} else if *targetURL != "" {
		// Create minimal config from CLI flags
		cfg = createConfigFromCLI(*targetURL, *output, *format)

		fmt.Println("⚙️  Using command-line arguments")
		fmt.Println()
	} else {
		// Try to load default config
		defaultConfig := "configs/crawler.yaml"
		if _, statErr := os.Stat(defaultConfig); statErr == nil {
			fmt.Printf("⚙️  Loading default configuration: %s\n", defaultConfig)

			cfg, err = config.LoadConfig(defaultConfig)
			if err != nil {
				log.Fatalf("❌ Failed to load default config: %v\n", err)
			}

			fmt.Printf("✅ Configuration loaded: %s\n\n", cfg)
		} else {
			log.Fatal("❌ Please provide -config file or -url flag, or place configs/crawler.yaml in working directory")
		}
	}

	printCrawlerHeader(cfg)

	// Create URL manager with fallback support
	urlManager := crawler.NewURLManager(cfg)
	scraper := crawler.NewScraperWithConfig(&cfg.Crawler.Retry, cfg.Advanced.BufferSizeKb)
	parser := parsers.NewParser()
	client := crawler.NewClientWithDeps(scraper, parser, urlManager)

	// Create validator
	markdownValidator, err := validator.NewMarkdownValidator(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create validator: %v\n", err)
	}

	// Process each enabled source
	enabledSources := cfg.GetEnabledSources()
	fmt.Printf("🚀 Processing %d enabled sources...\n", len(enabledSources))

	for i, sourceConfig := range enabledSources {
		fmt.Printf("\n----------------------------------------------------------------\n")
		fmt.Printf("📦 Source %d/%d: %s (%s/%s)\n", i+1, len(enabledSources), sourceConfig.Name, sourceConfig.FireID, sourceConfig.Language)

		// Create a temporary config with just this source to use existing URLManager logic
		sourceCfg := *cfg
		sourceCfg.Crawler.Sources = []config.SourceConfig{sourceConfig}

		urlManager := crawler.NewURLManager(&sourceCfg)

		// Fetch from source (with retries)
		var markdown string

		var fireID, language string

		var fetchSuccess bool

		for {
			source, sourceName, fID, lang, attemptNum, err := urlManager.NextURL()
			if err != nil {
				fmt.Printf("❌ Source exhausted: %v\n", err)

				break
			}

			// Check if this is a local file source
			if urlManager.IsCurrentSourceLocal() {
				// Ensure local data is up to date
				gitPull(source)

				fmt.Printf("⏳ Reading local file: %s\n", source)

				content, fileSize, duration, readErr := scraper.ReadLocalFileWithMetrics(source)
				urlManager.RecordAttempt(source, readErr == nil, readErr, 0, duration)

				if readErr == nil {
					markdown = content
					fireID = fID
					language = lang
					fetchSuccess = true

					fmt.Printf("✅ Successfully read %d bytes (%.2fms)\n", fileSize, float64(duration.Microseconds())/1000)

					break
				}

				fmt.Printf("❌ Failed to read local file: %v\n", readErr)

				continue
			}

			// Remote URL source
			fmt.Printf("⏳ Fetching (Attempt %d): %s\n   Remote: %s\n", attemptNum, sourceName, source)

			content, statusCode, duration, fetchErr := scraper.ScrapeWithMetrics(source)
			urlManager.RecordAttempt(source, fetchErr == nil, fetchErr, statusCode, duration)

			if fetchErr == nil {
				markdown = content
				fireID = fID
				language = lang
				fetchSuccess = true

				fmt.Printf("✅ Successfully fetched [Remote] from %s (%.2fs)\n", sourceName, duration.Seconds())

				break
			}

			fmt.Printf("❌ Failed: %v (%.2fs)\n", fetchErr, duration.Seconds())

			// Check if we should retry
			if attemptNum < cfg.Crawler.Retry.MaxAttempts {
				delay := urlManager.GetRetryDelay(attemptNum)
				fmt.Printf("⏳ Retrying in %.1f seconds...\n", delay.Seconds())
				// Note: NextURL will handle the retry increment
			}
		}

		if !fetchSuccess {
			fmt.Printf("⚠️  Skipping source %s due to fetch failure\n", sourceConfig.Name)

			continue
		}

		// Validate markdown format if requested or if required by config
		if *showValidation || (cfg.Crawler.Validation.ValidateTableFormat && cfg.Features.StrictValidation) {
			fmt.Println("\n🔍 Validating markdown format...")

			valResult := markdownValidator.ValidateMarkdown(markdown)

			if cfg.Crawler.Logging.DetailedValidation {
				valResult.PrintWarnings()

				if !valResult.IsValid {
					valResult.PrintErrors()
				}
			}

			fmt.Printf("%s\n", valResult)

			if !valResult.IsValid && cfg.Features.StrictValidation {
				fmt.Printf("❌ Validation failed in strict mode, skipping...\n")

				continue
			}
		}

		// Parse events
		fmt.Println("\n📊 Parsing timeline events...")

		events, err := parser.ParseMarkdownTable(markdown)
		if err != nil {
			fmt.Printf("❌ Parse failed: %v\n", err)

			continue
		}

		// Parse full document for additional metadata
		doc, docErr := parser.ParseDocument(markdown)
		if docErr != nil {
			fmt.Printf("⚠️  Could not parse document metadata: %v\n", docErr)
		}

		fmt.Printf("✅ Successfully extracted %d events\n", len(events))

		// If document metadata contains an IncidentID, use it to override the config ID
		// This allows dynamic directory structure based on content
		if doc != nil && doc.BasicInfo.IncidentID != "" {
			fmt.Printf("ℹ️  Found Incident ID in document: %s (overriding config: %s)\n",
				doc.BasicInfo.IncidentID, fireID)

			fireID = doc.BasicInfo.IncidentID
		}

		// Determine output path
		fmt.Println("\n📝 Saving to JSON...")

		outputPath := cfg.GetOutputPath(fireID, language)
		if *output != "" && len(enabledSources) == 1 {
			// Only override output path if processing single source or specified via CLI
			outputPath = *output
		}

		// Create backup if file exists
		if cfg.Crawler.Output.CreateBackup {
			if _, statErr := os.Stat(outputPath); statErr == nil {
				backupPath := outputPath + ".bak"
				if renameErr := os.Rename(outputPath, backupPath); renameErr != nil {
					fmt.Printf("⚠️  Could not create backup: %v\n", renameErr)
				} else {
					fmt.Printf("💾 Backed up existing file to: %s\n", backupPath)
				}
			}
		}

		// Ensure output directory exists
		outputDir := filepath.Dir(outputPath)
		if outputDir != "." && outputDir != "" {
			if mkdirErr := os.MkdirAll(outputDir, 0755); mkdirErr != nil {
				log.Fatalf("❌ Could not create output directory: %v\n", mkdirErr)
			}
		}

		// Save with document metadata if available
		if doc != nil {
			err = client.SaveTimelineJSONWithDocument(events, doc, outputPath)
		} else {
			err = client.SaveTimelineJSON(events, outputPath)
		}

		if err != nil {
			fmt.Printf("❌ Save failed: %v\n", err)

			continue
		}

		fmt.Printf("✅ Saved to: %s\n", outputPath)
	}

	fmt.Println("\n✨ Crawling complete!")
}

// createConfigFromCLI creates a config from CLI arguments.
func createConfigFromCLI(url, output, format string) *config.Config {
	cfg := &config.Config{
		Crawler: config.CrawlerConfig{
			Sources: []config.SourceConfig{
				{
					FireID:   "cli",
					FireName: "CLI Input",
					Language: "zh-hk",
					URL:      url,
					Name:     "CLI Argument",
					Enabled:  true,
				},
			},
			Retry: config.RetryPolicy{
				MaxAttempts:       3,
				InitialDelayMs:    500,
				MaxDelayMs:        30000,
				BackoffMultiplier: 2.0,
				TimeoutSec:        30,
			},
			Output: config.OutputConfig{
				BasePath:     "./data/fire",
				Path:         output,
				Format:       "json",
				PrettyPrint:  true,
				CreateBackup: true,
			},
			Validation: config.ValidationConfig{
				ValidateTableFormat: false,
				MinEvents:           0,
				MaxEvents:           500,
			},
			Logging: config.LoggingConfig{
				Level:              "info",
				ShowProgress:       true,
				SampleEvents:       3,
				DetailedValidation: false,
			},
		},
		Features: config.FeaturesConfig{
			StrictValidation: false,
		},
		Advanced: config.AdvancedConfig{
			BufferSizeKb:               1024,
			ContinueOnValidationErrors: true,
		},
	}

	if output == "" {
		cfg.Crawler.Output.Path = "data/timeline.json"
	}

	if format != "" {
		cfg.Crawler.Output.Format = format
	}

	return cfg
}

func printCrawlerHeader(cfg *config.Config) {
	fmt.Println("🕷️  TPWFC Timeline Crawler")
	fmt.Printf("Available sources: %d\n", len(cfg.GetEnabledSources()))
	fmt.Printf("Retry policy: max %d attempts, %.1fx backoff\n",
		cfg.Crawler.Retry.MaxAttempts,
		cfg.Crawler.Retry.BackoffMultiplier)
	fmt.Printf("Output: %s (%s format)\n", cfg.Crawler.Output.Path, cfg.Crawler.Output.Format)
	fmt.Println()
}

// runLocalFileMode handles crawling from a local markdown file.
func runLocalFileMode(filePath, outputPath string, validate bool) {
	fmt.Println("🕷️  TPWFC Timeline Crawler - Local File Mode")
	fmt.Printf("📂 Source file: %s\n", filePath)
	fmt.Println()

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("❌ Local file not found: %s\n", filePath)
	}

	// Create default config for local mode
	cfg := createConfigFromCLI("", outputPath, "json")

	// Create components
	scraper := crawler.NewScraperWithConfig(&cfg.Crawler.Retry, cfg.Advanced.BufferSizeKb)
	parser := parsers.NewParser()
	client := crawler.NewClientWithDeps(scraper, parser, nil)

	// Read local file with metrics
	fmt.Println("⏳ Reading local file...")

	markdown, fileSize, duration, err := scraper.ReadLocalFileWithMetrics(filePath)
	if err != nil {
		log.Fatalf("❌ Failed to read file: %v\n", err)
	}

	fmt.Printf("✅ Successfully read %d bytes (%.2fms)\n", fileSize, float64(duration.Microseconds())/1000)

	// Validate markdown format if requested
	if validate {
		fmt.Println("\n🔍 Validating markdown format...")

		markdownValidator, validatorErr := validator.NewMarkdownValidator(cfg)
		if validatorErr != nil {
			log.Fatalf("❌ Failed to create validator: %v\n", validatorErr)
		}

		valResult := markdownValidator.ValidateMarkdown(markdown)
		valResult.PrintWarnings()

		if !valResult.IsValid {
			valResult.PrintErrors()
		}

		fmt.Printf("%s\n", valResult)
	}

	// Parse events
	fmt.Println("\n📊 Parsing timeline events...")

	events, err := parser.ParseMarkdownTable(markdown)
	if err != nil {
		log.Fatalf("❌ Parse failed: %v\n", err)
	}

	// Parse full document for additional metadata
	doc, docErr := parser.ParseDocument(markdown)
	if docErr != nil {
		fmt.Printf("⚠️  Could not parse document metadata: %v\n", docErr)
	}

	fmt.Printf("✅ Successfully extracted %d events\n", len(events))

	// Determine output path
	if outputPath == "" {
		// Default output path based on input file
		dir := filepath.Dir(filePath)
		base := filepath.Base(filePath)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		outputPath = filepath.Join(dir, name+".json")
	}

	fmt.Println("\n📝 Saving to JSON...")

	// Create backup if file exists
	if cfg.Crawler.Output.CreateBackup {
		if _, statErr := os.Stat(outputPath); statErr == nil {
			backupPath := outputPath + ".bak"
			if renameErr := os.Rename(outputPath, backupPath); renameErr != nil {
				fmt.Printf("⚠️  Could not create backup: %v\n", renameErr)
			} else {
				fmt.Printf("💾 Backed up existing file to: %s\n", backupPath)
			}
		}
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." && outputDir != "" {
		if mkdirErr := os.MkdirAll(outputDir, 0755); mkdirErr != nil {
			log.Fatalf("❌ Could not create output directory: %v\n", mkdirErr)
		}
	}

	// Save with document metadata if available
	if doc != nil {
		err = client.SaveTimelineJSONWithDocument(events, doc, outputPath)
	} else {
		err = client.SaveTimelineJSON(events, outputPath)
	}

	if err != nil {
		log.Fatalf("❌ Save failed: %v\n", err)
	}

	fmt.Printf("✅ Saved to: %s\n", outputPath)

	// Print sample events
	sampleCount := 3
	if sampleCount > 0 {
		fmt.Printf("\n📊 Sample events (first %d):\n", sampleCount)

		for i := 0; i < sampleCount && i < len(events); i++ {
			event := events[i]
			fmt.Printf("  [%s] %s\n", event.DateTime, event.Description)

			if len(event.Casualties.Items) > 0 {
				fmt.Printf("    Casualties: ")
				for _, item := range event.Casualties.Items {
					fmt.Printf("%s: %d, ", item.Type, item.Count)
				}
				fmt.Println()
			}
		}
	}

	// Print statistics
	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("  Source: Local file (%s)\n", filePath)
	fmt.Printf("  Total events: %d\n", len(events))
	fmt.Printf("  Output format: json\n")
	fmt.Printf("  Output path: %s\n", outputPath)

	fmt.Println("\n✨ Local file crawling complete!")
}

func printUsage() {
	fmt.Println("Usage: ./bin/crawler [OPTIONS]")
	fmt.Println()
	fmt.Println("Modes:")
	fmt.Println("  1. Config-based:  ./bin/crawler -config configs/crawler.yaml")
	fmt.Println("  2. Default config: ./bin/crawler (reads configs/crawler.yaml if exists)")
	fmt.Println("  3. CLI arguments:  ./bin/crawler -url <URL> -output <PATH>")
	fmt.Println("  4. Local file:     ./bin/crawler -file <PATH> [-output <PATH>]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ./bin/crawler -config configs/crawler.yaml")
	fmt.Println("  ./bin/crawler -url https://raw.githubusercontent.com/... -output output.json")
	fmt.Println("  ./bin/crawler -file data/source/zh-HK/fire/WANG_FUK_COURT_FIRE_2025/timeline.md -output data/fire/WANG_FUK_COURT_FIRE_2025/zh-hk/timeline.json")
	fmt.Println("  ./bin/crawler -config configs/crawler.yaml -validate")
}

// gitPull executes git pull in the directory of the given file path.
func gitPull(filePath string) {
	// Resolve absolute path to be safe, or use relative if that's how we run
	dir := filepath.Dir(filePath)

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// If directory doesn't exist, we can't pull.
		// It might be created later or path is wrong.
		return
	}

	fmt.Printf("🔄 Checking for git updates in %s...\n", dir)

	// Create command
	cmd := exec.Command("git", "pull")
	cmd.Dir = dir

	// Run command
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log warning but don't fail the whole process - local file might still be readable
		fmt.Printf("⚠️  Git pull warning: %v\n", err)
	} else {
		// Only print output if verbose or interesting; git pull usually prints "Already up to date."
		// We'll print a summary.
		outputStr := string(output)
		if outputStr != "" {
			// Trim newline
			if outputStr[len(outputStr)-1] == '\n' {
				outputStr = outputStr[:len(outputStr)-1]
			}
			fmt.Printf("📄 Git output: %s\n", outputStr)
		}
	}
}
