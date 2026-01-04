.PHONY: help build run test clean

help:
	@echo "TPWFC Worker - Available commands:"
	@echo "  make build        - Build all executables"
	@echo "  make run-crawler  - Run crawler"
	@echo "  make run-normalizer - Run normalizer"
	@echo "  make run-worker   - Run combined worker"
	@echo "  make run-uploader - Run uploader"
	@echo "  make run-seed     - Run seeder (post-deploy data upload)"
	@echo "  make run-deploy   - Run deployment"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-up    - Start Docker Compose"
	@echo "  make docker-down  - Stop Docker Compose"

# Build all executables using Go directly
build:
	@mkdir -p bin
	@echo "Building TPWFC Worker..."
	go build -o bin/crawler ./cmd/crawler
	go build -o bin/normalizer ./cmd/normalizer
	go build -o bin/worker ./cmd/worker
	go build -o bin/uploader ./cmd/uploader
	go build -o bin/formatter ./cmd/formatter
	go build -o bin/signer ./cmd/signer
	go build -o bin/build ./cmd/build
	go build -o bin/deploy ./cmd/deploy
	go build -o bin/seed ./cmd/seed
	@echo "Build complete! Executables available in ./bin/"

run-crawler:
	./bin/crawler -url $(URL) -output ./data/raw-data.json

run-normalizer:
	./bin/normalizer -input $(INPUT) -output $(OUTPUT)

run-worker:
	./bin/worker -crawler-url $(CRAWLER_URL) -payload-url $(PAYLOAD_URL) -api-key $(API_KEY)

run-uploader:
	./bin/uploader --input $(INPUT) --endpoint $(ENDPOINT)

run-sign:
	./bin/signer -input $(INPUT)

run-formatter:
	./bin/formatter -path $(PATH)

run-seed:
	./bin/seed

run-deploy:
	./bin/deploy

test:
	go test -v ./...

clean:
	rm -rf bin/
	# Only remove ignored artifacts in data, preserving source/templates/go.mod
	rm -rf data/fire
	rm -f data/*.json
	rm -f data/raw-data.json
	go clean

docker-build:
	docker build -f deployments/docker/Dockerfile -t tpwfc-worker:latest .

docker-up:
	docker-compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	docker-compose -f deployments/docker/docker-compose.yml down

fmt:
	go fmt ./...

lint:
	golangci-lint run --fix

mod-tidy:
	go mod tidy