package benchmarks_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gatewatch/file_search/internal/cache"
	"gatewatch/file_search/internal/db"
)

func BenchmarkSearch_ColdCache(b *testing.B) {
	d := newBenchDB(b, 5_000)
	ctx := context.Background()
	queries := []string{"contrat", "facture", "avocat", "dossier", "rapport", "jugement"}
	samples := make([]int64, 0, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := queries[i%len(queries)]
		start := time.Now()
		_, _ = d.Search(ctx, q, 10, 0, db.SearchFilter{})
		samples = append(samples, time.Since(start).Nanoseconds())
	}
	reportPercentiles(b, samples)
}

func BenchmarkSearch_WarmCache(b *testing.B) {
	d := newBenchDB(b, 5_000)
	c := cache.New(128, 5*time.Minute)
	ctx := context.Background()
	results, _ := d.Search(ctx, "contrat", 10, 0, db.SearchFilter{})
	cacheResults := make([]cache.Result, len(results))
	for i, r := range results {
		cacheResults[i] = cache.Result{Path: r.Path, Snippet: r.Snippet}
	}
	c.Set("contrat||10||0||", cacheResults, len(results))
	samples := make([]int64, 0, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, _, _ = c.Get("contrat||10||0||")
		samples = append(samples, time.Since(start).Nanoseconds())
	}
	reportPercentiles(b, samples)
}

func BenchmarkSearch_Paginated(b *testing.B) {
	d := newBenchDB(b, 5_000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Search(ctx, "contrat", 10, (i%10)*10, db.SearchFilter{})
	}
}

func BenchmarkSearch_WithFilter(b *testing.B) {
	d := newBenchDB(b, 5_000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Search(ctx, "contrat", 10, 0, db.SearchFilter{Since: "today"})
	}
}

// BenchmarkSearch_SingleQuery vs DoubleQuery proves the double-FTS5 hotspot.
func BenchmarkSearch_SingleQuery(b *testing.B) {
	d := newBenchDB(b, 5_000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Search(ctx, "contrat", 10, 0, db.SearchFilter{})
	}
}

func BenchmarkSearch_DoubleQuery(b *testing.B) {
	d := newBenchDB(b, 5_000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Search(ctx, "contrat", 10, 0, db.SearchFilter{})
		_, _ = d.Count(ctx, "contrat", db.SearchFilter{})
	}
}

// BenchmarkCache_InvalidateByPath proves the O(n*m) hotspot across cache sizes.
func BenchmarkCache_InvalidateByPath_10entries(b *testing.B)  { benchInvalidate(b, 10) }
func BenchmarkCache_InvalidateByPath_64entries(b *testing.B)  { benchInvalidate(b, 64) }
func BenchmarkCache_InvalidateByPath_128entries(b *testing.B) { benchInvalidate(b, 128) }

func benchInvalidate(b *testing.B, entries int) {
	b.Helper()
	c := cache.New(entries, 5*time.Minute)
	resultSet := []cache.Result{
		{Path: "/bench/doc_000.txt", Snippet: "contrat"},
		{Path: "/bench/doc_001.txt", Snippet: "facture"},
		{Path: "/bench/doc_002.txt", Snippet: "avocat"},
	}
	for i := 0; i < entries; i++ {
		c.Set(fmt.Sprintf("query_%d||10||0||", i), resultSet, len(resultSet))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.InvalidateByPath("/bench/doc_000.txt")
		// Refill so every iteration has the same number of entries
		c.Set(fmt.Sprintf("refill_%d||10||0||", i%entries), resultSet, len(resultSet))
	}
}
