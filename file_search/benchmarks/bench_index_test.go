package benchmarks_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gatewatch/file_search/internal/db"
	"gatewatch/file_search/internal/indexer"
)

// BenchmarkIndex_TxtFiles_1K measures FULL indexing pipeline: file I/O + text extraction + FTS5 insert.
// This IS the end-to-end indexer throughput (files/s).
func BenchmarkIndex_TxtFiles_1K(b *testing.B)  { benchIndex(b, 1_000, "txt") }
func BenchmarkIndex_TxtFiles_10K(b *testing.B) { benchIndex(b, 10_000, "txt") }

func BenchmarkIndex_MixedFormats_1K(b *testing.B) { benchIndexMixed(b, 1_000) }

func benchIndex(b *testing.B, n int, ext string) {
	b.Helper()
	dir := b.TempDir()
	writeFiles(b, dir, n, ext)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dbPath := filepath.Join(b.TempDir(), "idx.db")
		d, err := db.New(context.Background(), dbPath)
		if err != nil {
			b.Fatal(err)
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		_, _ = indexer.Run(ctx, d, dir)
		cancel()
		elapsed := time.Since(start)
		_ = d.Close()
		if elapsed > 0 {
			b.ReportMetric(float64(n)/elapsed.Seconds(), "files/s")
		}
	}
}

func benchIndexMixed(b *testing.B, n int) {
	b.Helper()
	dir := b.TempDir()
	r := rand.New(rand.NewSource(77))
	exts := []string{"txt", "md", "html"}
	for i := 0; i < n; i++ {
		ext := exts[r.Intn(len(exts))]
		content := makeSentence(r, 30+r.Intn(20))
		path := filepath.Join(dir, fmt.Sprintf("doc_%04d.%s", i, ext))
		switch ext {
		case "html":
			content = "<html><body><p>" + content + "</p></body></html>"
		case "md":
			content = "# Titre\n\n" + content + "\n\n## Section\n\n" + makeSentence(r, 10)
		}
		_ = os.WriteFile(path, []byte(content), 0o644)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dbPath := filepath.Join(b.TempDir(), "idx.db")
		d, err := db.New(context.Background(), dbPath)
		if err != nil {
			b.Fatal(err)
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		_, _ = indexer.Run(ctx, d, dir)
		cancel()
		elapsed := time.Since(start)
		_ = d.Close()
		if elapsed > 0 {
			b.ReportMetric(float64(n)/elapsed.Seconds(), "files/s")
		}
	}
}

func writeFiles(b *testing.B, dir string, n int, ext string) {
	b.Helper()
	r := rand.New(rand.NewSource(55))
	for i := 0; i < n; i++ {
		content := makeSentence(r, 25+r.Intn(25))
		path := filepath.Join(dir, fmt.Sprintf("doc_%04d.%s", i, ext))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
}
