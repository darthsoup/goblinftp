# GoblinFTP task runner — https://just.systems
set dotenv-load

# ── Development ────────────────────────────────────────────────────────────────

# Start frontend + backend together (requires overmind: https://github.com/DarthSim/overmind)
dev:
	overmind start -f Procfile

# Start frontend dev server only
dev-fe:
	cd frontend && npm run dev

# Start backend dev server only
dev-be:
	cd backend && go run ./cmd/gftp

# ── Build ──────────────────────────────────────────────────────────────────────

# Build everything
build: build-fe build-be

# Build Nuxt SPA → frontend/.output/public/
build-fe:
	cd frontend && npm run generate

# Build Go binary → bin/gftp
build-be:
	mkdir -p bin
	cd backend && go build -o ../bin/gftp ./cmd/gftp

# ── Test ───────────────────────────────────────────────────────────────────────

# Run all tests
test: test-fe test-be

# Run frontend tests
test-fe:
	cd frontend && npm test

# Run backend tests
test-be:
	cd backend && go test ./...

# ── Lint / Format ──────────────────────────────────────────────────────────────

# Run all linters
lint: lint-fe lint-be

# Type-check frontend (no eslint config yet — added in Phase 5)
lint-fe:
	cd frontend && npm run typecheck

# Lint backend (requires golangci-lint: https://golangci-lint.run)
lint-be:
	cd backend && golangci-lint run ./...

# Format all code
fmt:
	cd frontend && npx prettier --write .
	cd backend && gofmt -w .

# ── Docker ─────────────────────────────────────────────────────────────────────

# Build Docker image
docker-build:
	docker build -t goblintools/gftp .

# Run Docker image
docker-run:
	docker run -p 8080:80 goblintools/gftp

# Push Docker image
docker-push:
	docker push goblintools/gftp

# Start with docker compose
docker-up:
	docker compose up --build

# Stop docker compose
docker-down:
	docker compose down

# ── Utilities ──────────────────────────────────────────────────────────────────

# Report i18n keys in en.json missing from de.json
i18n-check:
	node -e " \
	  const en = require('./frontend/i18n/locales/en.json'); \
	  const de = require('./frontend/i18n/locales/de.json'); \
	  const missing = Object.keys(en).filter(k => !(k in de)); \
	  if (missing.length) { console.log('Missing in de.json:', missing); process.exit(1); } \
	  else console.log('All keys present in de.json'); \
	"

# Remove build artifacts
clean:
	rm -rf frontend/.output frontend/node_modules bin/
