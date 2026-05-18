package benchmarks_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gatewatch/file_search/internal/cache"
	"gatewatch/file_search/internal/db"
	"gatewatch/file_search/internal/watcher"
)

// BenchmarkWatcher_Startup — cost of creating watcher + watching 1 directory.
func BenchmarkWatcher_Startup(b *testing.B) {
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dbPath := filepath.Join(b.TempDir(), "w.db")
		d, err := db.New(context.Background(), dbPath)
		if err != nil {
			b.Fatal(err)
		}
		c := cache.New(128, 5*time.Minute)
		w := watcher.New(dir, d, c)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		go w.Run(ctx)
		time.Sleep(10 * time.Millisecond)
		cancel()
		_ = d.Close()
	}
}

// BenchmarkWatcher_FileCreate_Latency — latency from file create to debounce flush.
// NOTE: This benchmark is slow by design: it waits for the 500ms debounce window.
// Run with -benchtime=10s and -count=1 for meaningful results.
func BenchmarkWatcher_FileCreate_Latency(b *testing.B) {
	dir := b.TempDir()
	dbPath := filepath.Join(b.TempDir(), "w2.db")
	d, err := db.New(context.Background(), dbPath)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = d.Close() })
	c := cache.New(128, 5*time.Minute)
	w := watcher.New(dir, d, c)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	b.Cleanup(cancel)
	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)
	samples := make([]int64, 0, b.N)
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))
		start := time.Now()
		_ = os.WriteFile(path, []byte("contrat facture dossier"), 0o644)
		time.Sleep(600 * time.Millisecond) // debounce window
		samples = append(samples, time.Since(start).Nanoseconds())
	}
	reportPercentiles(b, samples)
	b.ReportMetric(600, "ms/debounce")
}
