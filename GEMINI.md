# TPWFC Worker Service

This directory contains the **Worker Service** for the Tai Po Wang Fuk Court Fire Documentation Project (TPWFC). It is a backend application written in Go, responsible for data ingestion, processing, and synchronization with the main Payload CMS.

## Project Overview

The Worker Service operates as a pipeline to:

1. **Crawl** web sources for information.
2. **Normalize** unstructured data into a standardized format.
3. **Upload** the processed data to the Payload CMS via GraphQL.

It is designed to be run as standalone CLI tools or as a containerized service within a Kubernetes cluster.

## Tech Stack

* **Language:** Go (v1.25+)
* **Build System:** Make & Bash scripts
* **Linting:** `golangci-lint`
* **Containerization:** Docker

## Directory Structure

* `cmd/`: Entry points for the various executables (`crawler`, `normalizer`, `uploader`, `worker`).
* `internal/`: Private application code (business logic, models, processors).
* `pkg/`: Public library code.
* `configs/`: Configuration files (YAML, env templates).
* `data/`: Data storage for raw sources, templates, and processed outputs.
* `deployments/`: Docker and Kubernetes configuration.
* `scripts/`: Build and deployment scripts.

## Building and Running

The project uses a `Makefile` to manage build and run tasks.

### Build

To build all binaries (`crawler`, `normalizer`, `uploader`, `worker`, `formatter`) into the `bin/` directory:

```bash
make build
```

### Running Components

Each component can be run individually using `make` commands or by executing the binaries in `bin/` directly.

* **Crawler:**

    ```bash
    make run-crawler URL="https://example.com/source"
    ```

* **Normalizer:**

    ```bash
    make run-normalizer INPUT="./data/source/input.md" OUTPUT="./data/fire/output.json"
    ```

* **Uploader:**

    ```bash
    make run-uploader INPUT="./data/fire/timeline.json" ENDPOINT="http://localhost:3000/api/graphql"
    ```

* **Full Worker Pipeline:**

    ```bash
    make run-worker CRAWLER_URL="..." PAYLOAD_URL="..." API_KEY="..."
    ```

* **Formatter:**

    ```bash
    make run-formatter PATH="./data/source/..."
    ```

### Docker

* **Build Image:** `make docker-build`
* **Start Services:** `make docker-up` (uses `docker-compose`)
* **Stop Services:** `make docker-down`

## Development Conventions

* **Testing:** Run all unit tests with verbose output:

    ```bash
    make test
    ```

* **Formatting:** Format code using standard Go formatting:

    ```bash
    make fmt
    ```

* **Linting:** Run linter with auto-fix enabled:

    ```bash
    make lint
    ```

* **Dependency Management:** Clean up `go.mod` and `go.sum`:

    ```bash
    make mod-tidy
    ```
