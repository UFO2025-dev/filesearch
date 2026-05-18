# 🔍 FileSearch

> **Offline. Private. Fast.**  
> Full-text + semantic search engine for your local files — no cloud, no telemetry, no subscription.

[![CI](https://github.com/UFO2025-dev/gatewatch_mvp/actions/workflows/ci.yml/badge.svg)](https://github.com/UFO2025-dev/gatewatch_mvp/actions/workflows/ci.yml)
[![Go 1.24](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform: Windows](https://img.shields.io/badge/Platform-Windows%20x64-0078D4?logo=windows)](https://github.com/UFO2025-dev/gatewatch_mvp/releases)

---

## Why this exists

Windows Search is slow, unreliable, and indexes your files into an opaque system database.  
Everything is great for filenames — but searches **inside** documents? Nothing.  
Cloud AI search tools send your documents to remote servers.

**FileSearch** indexes the content of your files locally, responds in milliseconds, and never leaves your machine.

Built for lawyers, doctors, researchers, and SMBs who need to search thousands of documents without trusting a third party.

---

## Key Features

| Feature | Status |
|---|---|
| Full-text search (FTS5) | ✅ Production |
| Live file watcher (fsnotify) | ✅ Production |
| SHA-256 skip-unchanged | ✅ Production |
| LRU search cache (128 entries, 30s TTL) | ✅ Production |
| Hardware-adaptive mode (Essential / Advanced / Pro) | ✅ Production |
| Secrets hard-exclusion (.ssh, .env, .pem, .aws…) | ✅ Production |
| OneDrive / cloud-placeholder skip | ✅ Production (Windows syscall) |
| Rate limiting (token bucket, 100 req/s/IP) | ✅ Production |
| CSRF protection | ✅ Production |
| Bearer token auth | ✅ Production |
| Panic recovery middleware | ✅ Production |
| Audit log (search queries, file opens) | ✅ Production |
| Diagnostics ZIP bundle | ✅ Production |
| Schema migrations (PRAGMA user_version) | ✅ v0→v3 |
| Log rotation (10 MB, 3 backups) | ✅ Production |
| Auto-update check (GitHub releases API) | ✅ Production |
| EN / FR UI language toggle | ✅ Production |
| Semantic search (cosine similarity, embeddings) | ⚠️ Partial — brute-force, RAM-limited |
| Signed Windows installer | 🔲 Roadmap |
| DB encryption (AES-256) | 🔲 Roadmap |
| ANN semantic search (sqlite-vec) | 🔲 Roadmap |

---

## Benchmarks (real, reproducible)

Machine: Intel i5-4200U @ 1.60GHz · 4 cores · WSL2 Ubuntu  
Source: [`benchmarks/results/baseline_linux.txt`](benchmarks/results/baseline_linux.txt)

| Benchmark | Result | Notes |
|---|---|---|
| Search warm cache | **270 ns/op** (p50: 93ns) | Zero allocations |
| FTS5 search · 1K files | 3.4 ms/op | 88 allocs, 5.9 KB |
| FTS5 COUNT · 10K files | 1.1 ms/op | 24 allocs |
| SQLite upsert (new file) | 776 µs/op | ~1,290 DB writes/sec |
| SQLite re-upsert (unchanged) | 121 µs/op | Hash match skip |
| FileCount (COUNT*) · 10K | **41 µs/op** | 376 B — O(1) |
| AllPaths (memory hotspot) | 13.4 ms · 1.2 MB | ⚠️ Replaced by FileCount |
| Cache invalidate · 128 entries | 1.07 µs/op | O(n·m) known issue |

> Benchmarks use `go test -bench` with `-benchmem`. Run them yourself:
> ```bash
> cd file_search
> go test -bench=. ./benchmarks/... -run ^$ -benchmem
> ```

---

## Supported File Formats

`.txt` `.md` `.html` `.csv` `.json` `.yaml` `.toml` `.xml` `.log` `.ini` `.cfg`  
`.pdf` `.docx` `.xlsx` `.pptx` `.odt` `.ods` `.odp`  
`.py` `.go` `.js` `.ts` `.rs` `.c` `.cpp` `.java` `.sql` `.sh` `.bat`

---

## Installation

### Windows (recommended)

1. Download `filesearch-vX.X.X-windows-amd64.exe` from [Releases](https://github.com/UFO2025-dev/gatewatch_mvp/releases)
2. Double-click — your browser opens automatically at `http://localhost:7890`
3. Drag and drop a folder to start indexing

> ⚠️ Windows SmartScreen may warn about an unknown publisher. Click "More info → Run anyway".  
> Code signing is on the roadmap (v1.3.0).

### Build from source

```bash
git clone https://github.com/UFO2025-dev/gatewatch_mvp
cd gatewatch_mvp/file_search
go build -ldflags="-s -w -X main.Version=dev" -o filesearch ./cmd/server/
./filesearch
```

Requirements: Go 1.24+, no CGO, no system dependencies.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   cmd/server/main.go                │
│  Version injection · Hardware detect · Lifecycle    │
└────────────────────┬────────────────────────────────┘
                     │
         ┌───────────▼───────────┐
         │  internal/server/     │  HTTP server (stdlib net/http)
         │  Middleware chain:    │  Recovery → CSRF → RateLimit → Auth
         │  12 routes / handlers │
         └──┬─────────────┬──────┘
            │             │
   ┌────────▼──┐    ┌──────▼──────┐
   │ FTS5      │    │  Semantic   │
   │ Search    │    │  Search     │
   │ (SQLite)  │    │  (cosine,   │
   │           │    │   in-RAM)   │
   └────┬──────┘    └─────────────┘
        │
   ┌────▼──────────────────────────────┐
   │  internal/db/db.go                │
   │  SQLite FTS5 · WAL · 32MB cache   │
   │  PRAGMA user_version migrations   │
   │  Upsert · Search · FileCount      │
   │  Audit log · AuditCSV             │
   └────┬──────────────────────────────┘
        │
   ┌────▼──────┐    ┌─────────────────┐
   │ Indexer   │    │  Watcher        │
   │ Worker    │    │  fsnotify       │
   │ Pool (≤8) │    │  500ms debounce │
   │ SHA-256   │    │  ≤10K dirs      │
   │ skip-     │    └─────────────────┘
   │ unchanged │
   └───────────┘
```

**Zero external service dependencies.** Everything runs in a single binary.

---

## Privacy Model

- **No network calls** except: optional GitHub release check (can be disabled), no document content ever leaves the machine
- **No telemetry** — zero opt-out required
- **Secrets hard-excluded**: `.ssh/`, `.gnupg/`, `.aws/`, `.azure/`, `.kube/`, `.env*`, `*.pem`, `*.key`, `*.p12`, `*.pfx` — enforced in `internal/indexer/indexer.go`, non-configurable
- **OneDrive online-only files skipped** — no accidental network downloads (Windows `GetFileAttributes` syscall)
- **DB stored locally**: `%APPDATA%\FileSearch\index.db` (Windows) / `~/.config/filesearch/index.db` (Linux)

---

## vs. Everything / Windows Search

| | FileSearch | Everything | Windows Search |
|---|---|---|---|
| Filename search | ✅ | ✅ ⚡ faster | ✅ |
| **Content search** | ✅ | ❌ | ✅ slow |
| **Semantic search** | ⚠️ partial | ❌ | ❌ |
| PDF / DOCX content | ✅ | ❌ | ✅ slow |
| Privacy / offline | ✅ 100% | ✅ | ⚠️ MS cloud |
| Open source | ✅ | ❌ | ❌ |
| Audit log | ✅ | ❌ | ❌ |
| API | ✅ REST | ❌ | ❌ |
| Cross-platform | Linux + Win | Win only | Win only |

---

## Roadmap

| Version | Features |
|---|---|
| v1.2 *(current)* | i18n EN/FR · Diagnostics bundle · OneDrive skip · CI/CD |
| v1.3 | Inno Setup installer · Self-signed binary · AES-256 DB encryption |
| v1.4 | ANN semantic search (sqlite-vec) · 100K+ doc scalability |
| v2.0 | Multi-user tokens · Enterprise RBAC · TLS |

---

## FAQ

**Q: Does it read my files in the cloud?**  
A: No. The binary has no HTTP client except the optional GitHub version check.

**Q: What happens if the DB corrupts?**  
A: Delete `index.db` — it rebuilds automatically on next launch. DB repair tooling is on the roadmap.

**Q: Can I use it on a network drive?**  
A: UNC paths (`\\server\share`) are rejected by the path validator. Local drives only.

**Q: Is semantic search production-ready?**  
A: Not yet. Current implementation is cosine brute-force in RAM. Suitable for ≤20K docs. ANN indexing (sqlite-vec) is roadmap v1.4.

**Q: Token auth — is it enabled by default?**  
A: No. Pass `-token=yourtoken` at launch to enable Bearer auth. Suitable for single-user desktop; enterprise multi-user is roadmap v2.0.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md). To report a vulnerability: open a GitHub Security Advisory (private).

## License

MIT — see [LICENSE](LICENSE).
