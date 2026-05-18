# FileSearch — Technical Audit Report

*Based on static analysis of source code at HEAD. All claims cite specific files and line numbers.*

---

## Executive Summary

FileSearch is a Go 1.24 local-only file search engine. Core FTS5 search, indexing, caching, and middleware are **production-quality** code. Semantic search and installer are incomplete. The gap between "working tool" and "sellable product" is mainly: installer signing, DB encryption, and ANN-scaled semantic search.

**Overall verdict**: Solid engineering foundation. Product packaging incomplete.

---

## Architecture Diagram

```
cmd/server/main.go
  Version injection (-ldflags -X main.Version)
  Hardware detection (internal/hardware)
  Lifecycle: start → wait → open browser → update check loop

internal/server/server.go  (12 routes)
  Middleware: Recovery → CSRF → RateLimit(100/s) → Auth(Bearer)
  Routes: /, /api/search, /api/semantic, /api/index, /api/status,
          /api/config, /api/version, /api/diagnostics/bundle,
          /api/audit, /health, /api/open, /static/*

internal/db/db.go
  SQLite FTS5 (unicode61 tokenizer), WAL mode, 32MB cache, 256MB mmap
  PRAGMA user_version migrations: v0→v1(FTS5+shadow), v2(hash col), v3(size col)
  currentSchemaVersion = 3

internal/indexer/indexer.go
  Worker pool: min(runtime.NumCPU(), 8) goroutines
  SHA-256 hash skip (unchanged files)
  Secrets hard-exclusion (non-configurable)
  Supported extensions: txt, md, pdf, docx, xlsx, odt, go, py, js, …

internal/watcher/watcher.go
  fsnotify v1.10.1
  500ms debounce per path
  Max 10,000 watched directories

internal/cache/cache.go
  hashicorp/golang-lru/v2 v2.0.7
  LRU + TTL (default 128 entries, 30s)
  Thread-safe with RWMutex
  InvalidateByPath on file change (O(n·m))
```

---

## What Exists Today (Verified)

| Component | File | Status | Notes |
|---|---|---|---|
| FTS5 full-text search | `internal/db/db.go` | ✅ Production | unicode61, WAL, 32MB cache |
| Schema migrations | `internal/db/db.go:66` | ✅ Production | PRAGMA user_version v0→v3 |
| File watcher | `internal/watcher/watcher.go` | ✅ Production | fsnotify, 500ms debounce |
| Indexer worker pool | `internal/indexer/indexer.go` | ✅ Production | ≤8 workers, SHA-256 skip |
| Secrets exclusion | `internal/indexer/indexer.go:66-120` | ✅ Production | Non-configurable |
| OneDrive skip | `internal/indexer/onedrive_windows.go` | ✅ Production | GetFileAttributes syscall |
| LRU cache | `internal/cache/cache.go` | ✅ Production | 270ns warm-path, 0 allocs |
| Rate limiter | `internal/server/middleware.go` | ✅ Production | Token bucket, 100/s/IP |
| CSRF protection | `internal/server/middleware.go:126` | ✅ Production | Header-based |
| Bearer auth | `internal/server/middleware.go:196` | ✅ Production | Opt-in via -token flag |
| Panic recovery | `internal/server/middleware.go:103` | ✅ Production | No stack leak to client |
| Audit log | `internal/db/audit.go` | ✅ Production | Search queries + opens |
| Diagnostics bundle | `internal/server/server.go` (handleDiagnosticsBundle) | ✅ Production | ZIP: info+config+audit+log |
| Hardware detection | `internal/hardware/hardware.go` | ✅ Production | 3 modes: Essential/Advanced/Pro |
| Log rotation | `internal/logger/logger.go` | ✅ Production | 10MB max, 3 backups (Windows) |
| Auto-update check | `cmd/server/main.go:311` | ✅ Production | GitHub releases API polling |
| Version injection | `cmd/server/main.go:35` | ✅ Production | -ldflags -X main.Version |
| EN/FR i18n | `internal/server/static/index.html` | ✅ Production | localStorage, 60+ keys |
| Benchmark suite | `benchmarks/` | ✅ Production | 6 files, baseline committed |
| CI / govulncheck | `.github/workflows/ci.yml` | ✅ Production | 3 jobs: build+test+vuln |
| Release workflow | `.github/workflows/release.yml` | ✅ Production | .exe + Linux + checksums |
| Semantic search | `internal/embedder/`, `internal/db/vectors.go` | ⚠️ Partial | Cosine brute-force, RAM-only |
| Path validation | `internal/server/middleware.go` | ✅ Production | UNC + OS paths blocked |

---

## Performance Evidence

All from `benchmarks/results/baseline_linux.txt` (i5-4200U, WSL2):

- Warm cache hit: **270ns, 0 allocs**
- FTS5 search 1K corpus: **3.4ms**
- FileCount (COUNT*): **41µs** — O(1) shadow table
- DB upsert (new): **776µs** (~1,290 writes/sec — DB only)
- AllPaths 10K (memory): **13.4ms, 1.2MB** — replaced by FileCount in UI

---

## Security Status

| Control | Status |
|---|---|
| Secrets exclusion (.ssh, .env, .pem…) | ✅ Non-configurable |
| Path traversal prevention | ✅ UNC + OS dirs blocked |
| Rate limiting | ✅ 100 req/s/IP |
| CSRF | ✅ Header-based |
| Panic recovery | ✅ No leak |
| Auth | ✅ Optional Bearer (-token flag) |
| govulncheck | ✅ CI on every push |
| DB encryption | ❌ Roadmap v1.3 |
| Code signing | ❌ Roadmap v1.3 |
| TLS | ❌ Roadmap v2.0 |

---

## Known Limitations (Technical Debt)

| Issue | Severity | Location | Fix |
|---|---|---|---|
| Semantic search brute-force | 🔴 High | `internal/embedder/` | sqlite-vec ANN |
| DB unencrypted at rest | 🔴 High | `internal/db/db.go` | AES-256 machine-bound |
| No installer/signing | 🟠 Medium | N/A | Inno Setup + certificate |
| Cache invalidation O(n·m) | 🟡 Low | `internal/cache/cache.go:98` | path→entries reverse index |
| AllPaths still O(n) memory | 🟡 Low | `internal/db/db.go:368` | Already replaced in UI, not removed |
| Logger Linux → stderr only | 🟡 Low | `internal/logger/logger.go:34` | Fine for desktop use |
| Auth off by default | 🟡 Low | `internal/server/middleware.go:196` | By design; document clearly |
| No auto-update mechanism | 🟠 Medium | `cmd/server/main.go:311` | Only detects, no download |

---

## Honest Scoring

| Dimension | Score | Notes |
|---|---|---|
| Architecture | 8/10 | Clean packages, good separation, zero CGO |
| Performance | 7/10 | FTS5 solid; semantic not scalable yet |
| Security | 6/10 | Good basics; DB encryption + signing missing |
| UX | 6/10 | Functional; onboarding sparse; no installer |
| Windows readiness | 6/10 | Binary works; no signed installer, no update |
| Enterprise readiness | 3/10 | Single-user; no RBAC; no TLS; no audit trails for compliance |
| Business readiness | 4/10 | No installer; no signing; no moat beyond privacy positioning |
