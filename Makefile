.PHONY: build build-all build-monolith test lint dev run-gateway run-user run-product run-cart run-order \
       migrate-up migrate-down migrate-create sqlc docker-up docker-down docker-build \
       docker-logs health smoke stress seed generate tidy

# ── Variables ──
GO := go
GOFLAGS := -v
BINARY_DIR := bin

# ── Build ──
build-all: build-gateway build-user build-product build-cart build-order build-migrate build-monolith

build-gateway:
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/gateway ./cmd/gateway

build-user:
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/user ./cmd/user

build-product:
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/product ./cmd/product

build-cart:
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/cart ./cmd/cart

build-order:
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/order ./cmd/order

build-migrate:
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/migrate ./cmd/migrate

build-monolith:
	$(GO) build $(GOFLAGS) -o $(BINARY_DIR)/monolith ./cmd/monolith

build:
	$(GO) build ./...

# ── Run (local dev) ──
dev:
	$(GO) run ./cmd/monolith

# Air 热重载：监听文件变化自动重新编译+重启（需先 go install github.com/air-verse/air@latest）
# Air hot-reload: watches file changes and auto-rebuilds+restarts (requires: go install github.com/air-verse/air@latest)
dev-live:
	air

run-gateway:
	$(GO) run ./cmd/gateway

run-user:
	$(GO) run ./cmd/user

run-product:
	$(GO) run ./cmd/product

run-cart:
	$(GO) run ./cmd/cart

run-order:
	$(GO) run ./cmd/order

# ── Test ──
test:
	$(GO) test ./... -v -count=1

test-short:
	$(GO) test ./... -short -v

test-coverage:
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html

test-integration:
	$(GO) test ./... -tags=integration -v -count=1

test-race:
	$(GO) test ./... -race -v

bench:
	$(GO) test ./... -bench=. -benchmem

# ── Lint ──
lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

# ── Code Generation ──
sqlc:
	cd internal/database && sqlc generate

generate: sqlc

# ── Database Migration ──
migrate-up:
	$(GO) run ./cmd/migrate -direction up

migrate-down:
	$(GO) run ./cmd/migrate -direction down

migrate-create:
	@read -p "Migration name: " name; \
	goose -dir internal/database/migrations create $${name} sql

# ── Seed ──
seed:
	$(GO) run ./scripts/seed.go

# ── Docker ──
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-build:
	docker compose build

docker-logs:
	docker compose logs -f

# ── Operations ──
smoke:
	bash scripts/smoke-test.sh http://localhost:80

stress:
	$(GO) run scripts/stress-test.go 100 http://localhost:80

health:
	@curl -s -X POST http://localhost/health | jq .

stock-sync:
	$(GO) run scripts/stock-sync.go

# ── Utilities ──
tidy:
	$(GO) mod tidy

vet:
	$(GO) vet ./...

clean:
	rm -rf $(BINARY_DIR)/ coverage.out coverage.html
