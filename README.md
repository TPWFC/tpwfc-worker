# TPWFC Worker Service

This repository contains the **Worker Service** for the Tai Po Wang Fuk Court Fire Documentation Project (TPWFC). It is a backend application written in Go, responsible for data ingestion, processing, and synchronization with the Payload CMS.

## Architecture

The worker service operates as a pipeline:

1. **Crawl**: Fetch data from external sources.
2. **Normalize**: Transform unstructured markdown/html into standardized JSON.
3. **Sign**: Validate structure and update metadata (hash/timestamp) for documentation files.
4. **Upload**: Push processed data to Payload CMS via GraphQL.

## Directory Structure

- `cmd/`: Entry points for executables.
  - `crawler`: Scraping logic.
  - `normalizer`: Markdown-to-JSON transformation.
  - `uploader`: GraphQL sync logic.
  - `signer`: Metadata signing and validation tool.
  - `worker`: Unified pipeline runner.
- `internal/`: Private application code (business logic, models).
- `pkg/`: Public library code (utilities, metadata handling).
- `configs/`: YAML configuration files.
- `data/`: Local storage for artifacts.
- `deployments/`: Docker and Kubernetes configurations.

## Getting Started

### Prerequisites

- Go 1.21+
- Make

### Build

To build all binaries into the `bin/` directory:

```bash
make build
```

## Usage

### 1. Crawling Data

Fetches raw content from a URL:

```bash
# Using Make
make run-crawler URL="https://example.com/source"

# Direct execution
./bin/crawler -url "https://example.com/source" -output ./data/raw.json
```

### 2. Normalizing Data

Transforms markdown files into structured JSON for the uploader:

```bash
# Using Make
make run-normalizer INPUT="./data/source/timeline.md" OUTPUT="./data/fire/output.json"

# Direct execution
./bin/normalizer -input ./data/source/timeline.md -output ./data/fire/output.json
```

### 3. Signing Documentation (New)

Validates the structure of a markdown file and updates its metadata block. This is required for data integrity and change detection.

```bash
# Using Make
make run-sign INPUT="./data/source/zh-HK/fire/WANG_FUK_COURT_FIRE_2025/timeline.md"

# Direct execution
./bin/signer -input ./data/source/zh-HK/fire/WANG_FUK_COURT_FIRE_2025/timeline.md
```

The metadata block looks like this:

```markdown
<!-- METADATA_START
VALIDATION: TRUE
LAST_MODIFY: 2025-12-20T18:00:00Z
HASH: <sha256_hash>
METADATA_END -->
```

### 4. Uploading to CMS

Synchronizes JSON data with the Payload CMS.

#### Standard Upload (Timeline)

```bash
./bin/uploader \
    --input "./data/fire/output.json" \
    --endpoint "http://localhost:3000/api/graphql" \
    --fire-id "WANG_FUK_COURT_FIRE_2025" \
    --fire-name "Wang Fuk Court Fire" \
    --map-name "Google Map" \
 --map-url "https://maps.app.goo.gl/..." \
    --language "zh-hk"
```

**Flags:**

- `--input`: Path to the JSON file.
- `--endpoint`: GraphQL API URL.
- `--fire-id`: Unique identifier for the incident.
- `--fire-name`: Display name of the incident.
- `--map-name`: Label for the map link (e.g., "Google Maps").
- `--map-url`: URL for the map location.
- `--language`: Locale code (`zh-hk`, `zh-cn`, `en-us`).

#### Detailed Timeline Upload

```bash
./bin/uploader --mode detailed --input "./data/detailed.json" --incident-id 1
```

## Testing

Run all unit and integration tests:

```bash
make test
```
