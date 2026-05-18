package benchmarks_test

import (
	"context"
	"fmt"
	"testing"

	"gatewatch/file_search/internal/db"
)

// BenchmarkDB_Upsert measures SQLite write throughput (FTS5 INSERT + shadow table).
// This is NOT indexer throughput — it excludes file I/O, hashing, and text extraction.
func BenchmarkDB_Upsert(b *testing.B) {
	d := newBenchDB(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Upsert(context.Background(),
			fmt.Sprintf("/bench/file_%d.txt", i),
			"contrat client avocat facture dossier rapport document accord",
		)
	}
}

// BenchmarkDB_UpsertUpdate measures SQLite re-upsert cost when content changes.
func BenchmarkDB_UpsertUpdate(b *testing.B) {
	d := newBenchDB(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Upsert(context.Background(),
			fmt.Sprintf("/bench/doc_%04d.txt", i%100),
			"contrat client facture tribunal jugement appel accord",
		)
	}
}

func BenchmarkDB_Search_1K(b *testing.B)  { benchSearch(b, 1_000) }
func BenchmarkDB_Search_10K(b *testing.B) { benchSearch(b, 10_000) }
func BenchmarkDB_Search_50K(b *testing.B) { benchSearch(b, 50_000) }

func benchSearch(b *testing.B, n int) {
	b.Helper()
	d := newBenchDB(b, n)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Search(ctx, "contrat", 10, 0, db.SearchFilter{})
	}
}

// BenchmarkDB_Count — second FTS5 scan (hotspot: called after every Search).
func BenchmarkDB_Count_10K(b *testing.B) {
	d := newBenchDB(b, 10_000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Count(ctx, "contrat", db.SearchFilter{})
	}
}

// BenchmarkDB_AllPaths_10K vs BenchmarkDB_FileCount_10K — proves the RAM-waste hotspot.
// AllPaths loads every path into []string; FileCount does SELECT COUNT(*) only.
// Result on 10K corpus: AllPaths ~13 ms + 1.2 MB alloc vs FileCount ~41 µs + 376 B.
// AllPaths allocates []string for every path; FileCount does SELECT COUNT(*).
func BenchmarkDB_AllPaths_10K(b *testing.B) {
	d := newBenchDB(b, 10_000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		paths, _ := d.AllPaths(ctx)
		_ = len(paths)
	}
}

func BenchmarkDB_FileCount_10K(b *testing.B) {
	d := newBenchDB(b, 10_000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.FileCount(ctx)
	}
}

func BenchmarkDB_Delete(b *testing.B) {
	d := newBenchDB(b, b.N+1)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Delete(ctx, fmt.Sprintf("/bench/doc_%04d.txt", i%b.N))
	}
}
