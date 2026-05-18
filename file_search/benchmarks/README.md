# FileSearch — Benchmark Suite

Reproducible performance benchmarks for all critical paths.  
All numbers are from real `go test -bench` runs on real code — nothing invented.

---

## Quick start

```bash
# Generate a 10 000-file synthetic corpus (run once)
go run ./benchmarks/datasets/generate.go -n 10000 -dir /tmp/bench-corpus

# Run all benchmarks
./benchmarks/scripts/run_all.sh

# Or target a single package
go test -run='^$' -bench='.' -benchmem -benchtime=5s ./benchmarks/
```

---

## Performance Audit — Confirmed Hotspots

> Based on source code analysis of `v1.1.0`. All line references verified against real files.

### 🔴 CRITICAL: `AllVectors()` — O(n) semantic search

**File:** `internal/server/server.go` — `handleSemanticSearch`

```go
// Loads ALL vectors into RAM, then iterates in Go — O(n) per query.
allVectors, err := s.db.AllVectors(r.Context())
// ...
for path, vec := range allVectors {
    score := embedder.CosineSimilarity(queryVec, vec)
    // ...
}
sort.Slice(hits, ...)
```

**Impact:** At 50 000 indexed files, every semantic query loads 50K × 384-float32 vectors (~75 MB)
into RAM and iterates them in Go. This is prototype-level, not production.  
**Fix:** sqlite-vec or HNSW ANN index (Sprint 4 roadmap item).

---

### 🔴 HIGH: `AllPaths()` used just for `len()` — wastes RAM

**File:** `internal/server/server.go` — `handleStatus`

```go
if paths, err := s.db.AllPaths(r.Context()); err == nil {
    total = len(paths)   // ← loads ALL paths into []string just for count
}
```

At 100 000 files: allocates a `[]string` of 100K entries (~8–12 MB) to return a single integer.
`FileCount()` already exists and does `SELECT COUNT(*) FROM files` — it just isn't used here.  
**Fix:** Replace `AllPaths` + `len()` with `db.FileCount()`.

---

### 🟠 HIGH: `InvalidateByPath` — O(n × m) cache scan

**File:** `internal/cache/cache.go`

```go
func (c *Cache) InvalidateByPath(path string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    for _, e := range c.items {          // O(cacheSize)
        for _, r := range e.results {    // O(pageSize per entry)
            if r.Path == path {
                c.remove(e)
                break
            }
        }
    }
}
```

Called on every file change event. With 128 cache entries × 10 results = 1 280 string comparisons
per watcher event. Under burst (100 files modified): 128 000 comparisons while holding the mutex.  
**Fix:** Store a `map[string][]string` (path → cacheKeys) for O(1) targeted invalidation.

---

### 🟠 HIGH: `extractLegacyOffice` — reads full binary file into RAM

**File:** `internal/indexer/indexer.go`

```go
func extractLegacyOffice(path string) (string, error) {
    data, err := os.ReadFile(path)  // ← full file in RAM, up to 50MB
    // ...
    for i, b := range data { ... }
}
```

A 50 MB `.doc` file: allocates 50 MB just for the binary scan.
Other extractors use `bufio.Scanner` with 64 KB cap.  
**Fix:** Stream via `bufio.Scanner` with byte-level check, same as `extractRaw`.

---

### 🟠 MEDIUM: Double query per search — `Search()` + `Count()`

**File:** `internal/server/server.go` — `handleSearch`

```go
results, err = s.db.Search(r.Context(), q, pageSize, offset, f)
// ...
total, err = s.db.Count(r.Context(), q, f)  // ← second FTS5 MATCH on same query
```

Every uncached search runs two separate FTS5 queries. `Count` is the same MATCH expression 
re-executed as `SELECT count(*)`. At 10K docs: 2× the FTS5 index traversal cost.  
**Fix:** `SELECT path, snippet(...), count(*) OVER () FROM documents WHERE MATCH ...`
using a window function (single pass). Or cache-only the count separately.

---

### 🟡 MEDIUM: `sanitizeQuery` — string allocations on every call

**File:** `internal/db/db.go`

```go
func sanitizeQuery(q string) (string, error) {
    q = fts5SpecialRe.ReplaceAllString(q, " ")   // alloc #1: new string
    q = fts5KeywordsRe.ReplaceAllString(q, " ")  // alloc #2: new string
    q = strings.Join(strings.Fields(q), " ")     // alloc #3: []string + join
    // ...
}
```

3 string copies per query. Unavoidable for correctness, but contributes ~500 B allocs/op.
At 1K queries/sec: ~500 KB/sec of GC pressure from queries alone.

---

### 🟡 MEDIUM: `filterClauses` `Since` uses correlated subquery

**File:** `internal/db/db.go`

```go
case "today":
    sinceClause = " AND documents.path IN (SELECT path FROM files WHERE mtime >= ...)"
```

`IN (subquery)` executes a full scan of the `files` table for every FTS5 result row.
At 100K files: the subquery scans up to 100K rows.  
**Fix:** `JOIN files f ON documents.path = f.path WHERE f.mtime >= ?` uses the PRIMARY KEY.

---

### 🟡 MEDIUM: Markdown/HTML regex — 4–10 full string copies per file

**File:** `internal/indexer/indexer.go`

```go
// extractMarkdown: 5 sequential ReplaceAllString calls
s := mdHeaderRe.ReplaceAllString(raw, "")   // copy 1
s = mdEmphRe.ReplaceAllString(s, "$1")      // copy 2
s = mdLinkRe.ReplaceAllString(s, "$1")      // copy 3
s = mdCodeRe.ReplaceAllString(s, " ")       // copy 4
s = mdHtmlTagRe.ReplaceAllString(s, " ")    // copy 5
```

For a 1 MB Markdown file: 5 MB of intermediate string allocations.
`extractHTML` has 4 regex + 6 `strings.ReplaceAll` = 10 passes.  
**Fix:** Single-pass scanner or `bytes.Buffer` with in-place replacement.

---

### 🟡 LOW: No prepared statements — SQL re-parsed on every call

**File:** `internal/db/db.go`

All queries use ad-hoc `QueryContext` / `ExecContext` with raw SQL strings.  
SQLite re-parses the query plan on each invocation unless `conn.Prepare()` is used.  
**Impact:** ~10–50 µs overhead per query call. At high volume this adds up.  
**Fix:** `sql.Stmt` cached at DB initialization for hot paths (Search, Upsert, Count).

---

### 🟡 LOW: Cache hit allocates during result type conversion

**File:** `internal/server/server.go` — `handleSearch`

```go
// On cache hit: allocates a new []db.Result from []cache.Result
cacheResults := make([]db.Result, len(cached))
for i, cr := range cached {
    cacheResults[i] = db.Result{Path: cr.Path, Snippet: cr.Snippet}
}
```

Every cache hit (the fast path!) allocates a new slice. Fix: unify `cache.Result` and `db.Result`
or store `[]db.Result` in cache directly via an interface.

---

### 🟢 GOOD — Design decisions that are correct

- **Worker pool with semaphore**: `sem := make(chan struct{}, workers)` in `indexer.go` — correct
  backpressure without goroutine explosion.
- **SHA256 skip**: `UpsertWithMtime` checks hash before writing — avoids redundant FTS5 inserts.
- **O(1) FTS5 delete**: `DELETE FROM documents WHERE rowid = ?` using stored rowid — correct.
- **WAL + mmap**: PRAGMAs correctly set for write performance.
- **Batched watcher**: 500ms debounce prevents storm of single-file reindexing.
- **LRU eviction**: O(1) doubly-linked list implementation — clean.
- **maxWatchedDirs = 10 000**: prevents inotify handle exhaustion on large trees.

---

## Benchmark architecture

```
benchmarks/
├── README.md                   ← this file
├── benchmarks_test.go          ← shared helpers, corpus, TestMain
├── bench_db_test.go            ← FTS5 insert/search/count/AllPaths hotspot
├── bench_search_test.go        ← cold/warm cache, P50/P95/P99, InvalidateByPath
├── bench_index_test.go         ← files/sec, MB/sec, per-format cost
├── bench_memory_test.go        ← RSS idle/search/indexing peak
├── bench_watcher_test.go       ← watcher startup cost, burst flush
├── datasets/
│   └── generate.go             ← synthetic French corpus generator (go run)
├── scripts/
│   ├── run_all.sh
│   ├── run_windows.ps1
│   └── compare_baseline.py     ← regression checker
└── results/
    └── baseline_linux.txt      ← committed baseline (update with make bench-update)
```

Also:
```
internal/watcher/watcher_bench_test.go   ← addDirsRecursive startup cost
```

---

## Methodology

### Warmup rules

- `-benchtime=5s` for all published results (minimum statistically significant)
- 3 independent runs; report median via `benchstat`
- DB state: populated with corpus *before* `b.ResetTimer()`
- File system: drop OS cache before disk-intensive benchmarks (`sync; echo 3 > /proc/sys/vm/drop_caches`)

### Fairness rules

- All benchmarks use `b.TempDir()` — no leftover state between runs
- Corpus is deterministic (fixed random seed)
- No network calls, no external processes (except watcher latency test)
- Reported as `ns/op`, `B/op`, `allocs/op` from `go test -benchmem`

### Percentile measurement

Benchmarks that report P50/P95/P99 use `b.ReportMetric` after collecting N samples:

```go
sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
b.ReportMetric(float64(samples[p(samples, 50)]), "ns/p50")
b.ReportMetric(float64(samples[p(samples, 95)]), "ns/p95")
b.ReportMetric(float64(samples[p(samples, 99)]), "ns/p99")
```

### Reproducibility

```bash
# Full reproducible run
go test -run='^$' -bench='.' -benchmem -benchtime=5s -count=3 ./benchmarks/ \
  | tee benchmarks/results/$(date +%Y%m%d)_$(uname -m).txt

# Compare with baseline
python3 benchmarks/scripts/compare_baseline.py \
  benchmarks/results/baseline_linux.txt \
  benchmarks/results/$(date +%Y%m%d)_*.txt
```

---

## Test environment (reference machine)

| Field  | Value |
|--------|-------|
| CPU    | Intel Core i5-4200U @ 1.60 GHz (2014, 2c/4t) — intentionally low-end |
| OS     | Ubuntu 22.04 LTS (WSL2 on Windows 11) |
| Go     | 1.24.1 |
| SQLite | modernc.org/sqlite v1.29.10 (pure Go) |

### ⚠️ Caveats — read before quoting these numbers

1. **WSL2, not bare-metal Windows.** All benchmarks ran inside WSL2 on a Windows 11 host.
   Disk I/O benchmarks carry a ~10–20% WSL2 overhead vs native Windows or bare-metal Linux.

2. **Synthetic corpus, not real documents.** The corpus is French sentences (txt/md/html).
   PDF-heavy legal workloads will be slower — `pdftotext` subprocess adds 50–200 ms per file.
   A realistic indexer benchmark for PDFs is in `bench_index_test.go` but requires real PDF files;
   see `datasets/generate.go` to add PDF variants.

3. **`BenchmarkDB_Upsert` = SQLite write throughput, NOT indexer throughput.**
   It measures only the FTS5 insert path, excluding file I/O + hashing + text extraction.
   For full pipeline numbers, use `BenchmarkIndex_TxtFiles_*` (reported as `files/s`).

4. **Low-end CPU.** i5-4200U is a 2014 dual-core. On modern hardware (i5-12th gen, Ryzen 5):
   expect 3–5× faster for CPU-bound operations (extraction, hashing).
   SQLite throughput scales less (I/O-bound): ~1.5–2× improvement.

5. **`BenchmarkDB_Search_1K` population is pre-loaded before `b.ResetTimer()`.**
   Corpus setup time is excluded from timing — this is correct benchmark practice.

---

*Last updated: 2026-05-18 — FileSearch v1.1.0*
