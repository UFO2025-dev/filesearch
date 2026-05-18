package benchmarks_test

import (
	"context"
	"runtime"
	"testing"

	"gatewatch/file_search/internal/db"
)

func BenchmarkMemory_IdleDB(b *testing.B) {
	b.ReportAllocs()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nd := newBenchDB(b, 0)
		_ = nd
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	growth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	if growth < 0 {
		growth = 0
	}
	b.ReportMetric(float64(growth)/float64(b.N)/1024, "KB/heap-growth")
}

func BenchmarkMemory_Search_1K(b *testing.B)  { benchMemSearch(b, 1_000) }
func BenchmarkMemory_Search_10K(b *testing.B) { benchMemSearch(b, 10_000) }

func benchMemSearch(b *testing.B, n int) {
	b.Helper()
	d := newBenchDB(b, n)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Search(ctx, "contrat", 10, 0, db.SearchFilter{})
	}
}

func BenchmarkMemory_AllPaths_RAM(b *testing.B) {
	d := newBenchDB(b, 10_000)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		paths, _ := d.AllPaths(ctx)
		_ = len(paths)
	}
}

func BenchmarkMemory_FileCount_RAM(b *testing.B) {
	d := newBenchDB(b, 10_000)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.FileCount(ctx)
	}
}
