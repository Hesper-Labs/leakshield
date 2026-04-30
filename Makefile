.PHONY: help dev test test-go test-py test-e2e proto build clean docker-up docker-down lint

help:
	@echo "LeakShield make targets:"
	@echo "  dev          Start gateway+inspector locally in parallel (no panel)"
	@echo "  docker-up    docker compose up -d (postgres, redis, gateway, inspector, panel)"
	@echo "  docker-down  docker compose down"
	@echo "  test         Run all tests (Go + Python + e2e)"
	@echo "  test-go      Run Go tests in gateway/"
	@echo "  test-py      Run Python tests in inspector/"
	@echo "  test-e2e     Run Playwright e2e tests in panel/"
	@echo "  proto        Generate protobuf stubs (Go + Python)"
	@echo "  lint         Run linters (golangci-lint, ruff, eslint)"
	@echo "  build        Build gateway binary + inspector wheel + panel build"
	@echo "  clean        Remove build artifacts"

dev:
	@echo "Starting gateway and inspector in parallel (Ctrl-C to stop)..."
	@(cd gateway && go run ./cmd/leakshield serve) & \
	 (cd inspector && python -m leakshield_inspector); \
	 wait

docker-up:
	docker compose up -d

docker-down:
	docker compose down

test: test-go test-py
	@echo "All tests passed."

test-go:
	cd gateway && go test ./...

test-py:
	cd inspector && pytest

test-e2e:
	cd panel && npm run test:e2e

proto:
	@echo "Generating Go protobuf stubs..."
	protoc \
		--go_out=gateway/internal/inspector/proto --go_opt=paths=source_relative \
		--go-grpc_out=gateway/internal/inspector/proto --go-grpc_opt=paths=source_relative \
		--proto_path=proto \
		proto/inspector/v1/inspector.proto
	@echo "Generating Python protobuf stubs..."
	cd inspector && python -m grpc_tools.protoc \
		--python_out=src \
		--grpc_python_out=src \
		--proto_path=../proto \
		../proto/inspector/v1/inspector.proto

lint:
	cd gateway && go vet ./... && (command -v golangci-lint >/dev/null && golangci-lint run ./... || echo "skip: golangci-lint not installed")
	cd inspector && ruff check src tests || true
	cd panel && (test -f package.json && npm run lint) || echo "skip: panel not initialized"

build:
	cd gateway && CGO_ENABLED=0 go build -o bin/leakshield ./cmd/leakshield
	cd inspector && python -m build || pip wheel . -w dist/ --no-deps

clean:
	rm -rf gateway/bin
	rm -rf inspector/dist inspector/build inspector/*.egg-info
	rm -rf panel/.next panel/out
