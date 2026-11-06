# Developer shortcuts; each target wraps a script in scripts/ or samples/.
.PHONY: build dev dev-noauth dev-pg samples curl smoke test lint generate

DEV_KEY ?= pk_dev_local

build:
	go build -o bin/proteus-server ./cmd/proteus-server
	go build -o bin/proteus-client ./samples/client

# Run the server with the dev key $(DEV_KEY), jobs in memory.
dev:
	PROTEUS_DEV_KEY=$(DEV_KEY) scripts/dev.sh

# Run the server without API keys.
dev-noauth:
	scripts/dev.sh --no-auth

# Run the server with jobs in Postgres (compose service, or PROTEUS_DATABASE_URL).
dev-pg:
	PROTEUS_DEV_KEY=$(DEV_KEY) scripts/dev.sh --postgres

# Convert every sample against a running dev server.
samples:
	PROTEUS_API_KEY=$(DEV_KEY) go run ./samples/client samples

# Call every RPC with buf curl against a running dev server.
curl:
	PROTEUS_API_KEY=$(DEV_KEY) samples/buf-curl.sh

# Start the server, run all samples over gRPC, Connect, gRPC-Web and buf curl, stop it.
smoke:
	PROTEUS_DEV_KEY=$(DEV_KEY) scripts/smoke.sh

test:
	go vet ./...
	test -z "$$(gofmt -l .)"
	go test -race ./...

lint:
	buf lint

generate:
	buf generate
	storm generate internal/storage/store
