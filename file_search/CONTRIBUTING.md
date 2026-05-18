# Contributing to FileSearch

Thank you for your interest in contributing.

## Development Setup

```bash
git clone https://github.com/UFO2025-dev/gatewatch_mvp
cd gatewatch_mvp/file_search
go build ./...
go test ./...
```

Requirements: **Go 1.24+**, no CGO required (uses `modernc.org/sqlite` — pure Go SQLite).

## Project Structure

```
file_search/
  cmd/server/main.go          # Entry point, lifecycle management
  internal/
    cache/cache.go            # Thread-safe LRU cache with TTL
    config/config.go          # Config load/save (%APPDATA% / ~/.config)
    db/db.go                  # SQLite FTS5 schema, migrations, search
    db/audit.go               # Audit log (search queries, file opens)
    db/vectors.go             # Embedding vector storage
    embedder/                 # Local semantic embedding (optional)
    hardware/hardware.go      # CPU/GPU/RAM detection → mode selection
    indexer/indexer.go        # File walk, worker pool, secrets exclusion
    logger/logger.go          # Structured slog + log rotation
    paths/validate.go         # Directory validation (UNC, OS paths)
    server/server.go          # HTTP handlers + middleware chain
    server/middleware.go      # Recovery, CSRF, rate-limit, auth
    server/static/index.html  # Embedded single-page UI (EN/FR)
    watcher/watcher.go        # fsnotify live re-indexing
  benchmarks/                 # Reproducible go test -bench suite
  .github/workflows/          # CI (build+test+vet+govulncheck) + Release
```

## Guidelines

### Code
- Keep packages focused — one responsibility per package
- All public functions must have Go doc comments
- No CGO — the binary must compile with `CGO_ENABLED=0`
- Run `go vet ./...` before opening a PR
- Run `go test -race ./...` — all tests must pass with race detector

### Benchmarks
- If your change touches hot paths (search, indexer, cache), add or update benchmarks in `benchmarks/`
- Do not regress the baseline in `benchmarks/results/baseline_linux.txt` by >20%
- Update baseline if you deliberately improve performance

### Security
- Never add code that sends document content over the network
- Hard-exclusions in `internal/indexer/indexer.go` must not be user-configurable
- Path validation in `internal/server/middleware.go` must not be relaxed

### UI (`internal/server/static/index.html`)
- All user-visible strings must be in the `T = {fr: {}, en: {}}` i18n dictionary
- No external JS/CSS dependencies — single self-contained file

## Pull Request Process

1. Fork, create a branch: `git checkout -b feat/your-feature`
2. Make changes, add tests
3. `go test -race ./...` — must pass
4. `go vet ./...` — must pass
5. Open PR with a clear description of what and why

## What We Need Help With

- 🔲 Inno Setup installer script (`packaging/filesearch.iss`)
- 🔲 sqlite-vec ANN integration (`internal/db/`)
- 🔲 AES-256 DB encryption wrapper
- 🔲 More language translations (ES, DE, AR…)
- 🔲 Windows CI runner tests (currently Linux only in CI)
- 🔲 PDF extraction improvements (complex layouts)
