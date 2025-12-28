#!/bin/bash

# seed.sh - Seeder script for post-deploy data upload
# This script waits for the web service to be healthy, then uploads data

set -e

# Configuration (override via environment variables)
WEB_URL="${WEB_URL:-http://tpwfc-web:3000}"
GRAPHQL_ENDPOINT="${WEB_URL}/api/graphql"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-120}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[SEEDER]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[SEEDER]${NC} $1"; }
log_error() { echo -e "${RED}[SEEDER]${NC} $1"; }

wait_for_web() {
    local start_time=$(date +%s)
    log_info "Waiting for web service at ${WEB_URL}..."
    
    while true; do
        local http_code=$(curl -s -o /dev/null -w "%{http_code}" "${WEB_URL}" 2>/dev/null || echo "000")
        
        if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 400 ]; then
            log_info "Web service is ready! (HTTP $http_code)"
            sleep 5  # Extra time for full initialization
            return 0
        fi
        
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [ "$elapsed" -ge "$HEALTH_TIMEOUT" ]; then
            log_error "Web service failed to start within ${HEALTH_TIMEOUT}s"
            return 1
        fi
        
        echo -n "."
        sleep 2
    done
}

# Wait for web service
if ! wait_for_web; then
    log_error "Aborting seeding - web service not available"
    exit 1
fi

# Check required environment variables
if [ -z "$ADMIN_EMAIL" ] || [ -z "$ADMIN_PASSWORD" ]; then
    log_error "ADMIN_EMAIL and ADMIN_PASSWORD must be set"
    exit 1
fi

# Run formatter on source files
log_info "Formatting source markdown files..."
./bin/formatter -path "./data/source" -write 2>/dev/null || true

# Run crawler
log_info "Running crawler..."
./bin/crawler -config ./configs/crawler.yaml

# Upload zh-HK timeline
ZH_HK_JSON="./data/fire/WANG_FUK_COURT_FIRE_2025/zh-hk/timeline.json"
if [ -f "$ZH_HK_JSON" ]; then
    log_info "Uploading zh-HK timeline..."
    UPLOAD_OUTPUT=$(./bin/uploader \
        --input "$ZH_HK_JSON" \
        --endpoint "$GRAPHQL_ENDPOINT" \
        --email "$ADMIN_EMAIL" \
        --password "$ADMIN_PASSWORD" \
        --language "zh-hk")
    echo "$UPLOAD_OUTPUT"
    
    INCIDENT_ID=$(echo "$UPLOAD_OUTPUT" | grep -o "incidentId=[0-9]*" | cut -d'=' -f2 || echo "1")
    log_info "Captured incidentId: ${INCIDENT_ID:-1}"
fi

# Upload en-US timeline
EN_US_JSON="./data/fire/WANG_FUK_COURT_FIRE_2025/en-us/timeline.json"
if [ -f "$EN_US_JSON" ]; then
    log_info "Uploading en-US timeline..."
    ./bin/uploader \
        --input "$EN_US_JSON" \
        --endpoint "$GRAPHQL_ENDPOINT" \
        --email "$ADMIN_EMAIL" \
        --password "$ADMIN_PASSWORD" \
        --language "en-us"
fi

# Upload zh-CN timeline
ZH_CN_JSON="./data/fire/WANG_FUK_COURT_FIRE_2025/zh-cn/timeline.json"
if [ -f "$ZH_CN_JSON" ]; then
    log_info "Uploading zh-CN timeline..."
    ./bin/uploader \
        --input "$ZH_CN_JSON" \
        --endpoint "$GRAPHQL_ENDPOINT" \
        --email "$ADMIN_EMAIL" \
        --password "$ADMIN_PASSWORD" \
        --language "zh-cn"
fi

log_info "==========================================="
log_info "Seeding complete!"
log_info "==========================================="
