# Changelog

All notable changes to FileSearch are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- `GET /api/version` endpoint returning version, Go runtime, OS/arch
- Version badge in UI header (populated from `/health` on load)
- `GET /api/diagnostics` endpoint — downloads ZIP bundle (info.json, config.json, audit.csv, last 500 log lines)
- "Support & Diagnostics" section in Settings tab with download button
- OneDrive / cloud-storage placeholder detection on Windows: online-only files are skipped silently instead of triggering network downloads
- GitHub Actions CI: build + test (race detector) + Windows cross-compile + govulncheck security scan
- GitHub Actions Release: auto-publish `.exe` + Linux binary + checksums on `v*.*.*` tag

### Changed
- `/health` now goes through full middleware chain (rate limit + CSRF + panic recovery); auth explicitly excluded so startup checks still work
- `handleStatus` uses `FileCount()` instead of `AllPaths()+len()` — 325× cheaper (41µs vs 13ms at 10K files)

### Security
- Secrets exclusion policy in indexer: `.ssh`, `.gnupg`, `.aws`, `.azure`, `.kube`, wallets, `.env*`, `*.pem`, `*.key`, `*.p12`, `*.pfx`, etc. — hard-excluded, non-configurable

## [1.1.0] — Sprint 1 hardening

### Added
- Directory validation before indexing (`internal/paths/validate.go`)
- Rate limiter (token bucket, 30 req/s per IP, burst 15)
- CSRF middleware for POST endpoints
- Panic recovery middleware
- Audit log (search queries, file opens, settings changes)
- Update checker polling GitHub releases API
- Professional benchmark suite (`benchmarks/`) with CI regression check

### Fixed
- DB and config paths now use `%APPDATA%\FileSearch\` on Windows, `~/.config/filesearch/` on Linux
- Log rotation: 10MB max, 3 backups, Windows-native rotating writer

## [1.0.0] — Initial release

### Added
- Full-text search with SQLite FTS5
- Indexer with worker pool, SHA-256 skip-unchanged, 20+ file formats
- Watcher (fsnotify) for live re-indexing
- LRU cache for search results
- Hardware-adaptive mode detection (Essentiel / Avancé / Pro)
- Embedded UI (no external dependencies)
- Optional local semantic search with embeddings
