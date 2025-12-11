# TPWFC Worker Service

This repository contains the **Worker Service** for the Tai Po Wang Fuk Court Fire Documentation Project (TPWFC). It is a backend application written in Go, responsible for data ingestion, processing, and synchronization.

## Architecture

The worker service is designed to work in conjunction with the [tpwfc-data](../tpwfc-data) repository, which holds the source data and templates.

## Directory Structure

- `cmd/`: Entry points (`crawler`, `normalizer`, `uploader`, `worker`).
- `internal/`: Private application code.
- `pkg/`: Public library code.
- `configs/`: Configuration files.
- `data/`: Local storage for **processed** outputs (source data is in `../tpwfc-data`).
- `deployments/`: Docker/Kubernetes configs.

## Getting Started

### Prerequisites

- Go 1.25+
- Make

### Build

```bash
make build
```

### Running

The worker is configured to look for source data in `../tpwfc-data`. Ensure that repository is checked out at the same level as `tpwfc-worker`.

**Run Crawler:**

```bash
make run-crawler URL="..."
```

**Run Normalizer:**

```bash
make run-normalizer INPUT="../tpwfc-data/source/zh-HK/fire/WANG_FUK_COURT_FIRE_2025/timeline.md" OUTPUT="./data/fire/output.json"
```

**Run Tests:**

```bash
make test
```
