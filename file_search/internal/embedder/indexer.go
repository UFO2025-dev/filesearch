package embedder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// VectorDB is the interface for storing vectors (implemented by *db.DB).
type VectorDB interface {
	AllVectors(ctx context.Context) (map[string][]float32, error)
	UpsertVector(ctx context.Context, path string, vec []float32) error
}

// FileSource provides the list of files to embed.
type FileSource interface {
	AllPaths(ctx context.Context) ([]string, error)
}

// BackgroundIndexer embeds files in the background at low priority.
type BackgroundIndexer struct {
	client *Client
	vdb    VectorDB
	source FileSource
	delay  time.Duration
}

func NewBackgroundIndexer(client *Client, vdb VectorDB, source FileSource) *BackgroundIndexer {
	return &BackgroundIndexer{
		client: client,
		vdb:    vdb,
		source: source,
		delay:  2 * time.Second,
	}
}

// Run starts the background embedding loop. Call in a goroutine.
// It waits for initialDelay before starting (to let FTS5 indexing finish first).
func (b *BackgroundIndexer) Run(ctx context.Context, initialDelay time.Duration) {
	slog.Info("embedder: background indexer waiting", "delay", initialDelay)
	select {
	case <-ctx.Done():
		return
	case <-time.After(initialDelay):
	}

	if err := b.client.Ping(ctx); err != nil {
		slog.Warn("embedder: ollama not available, skipping semantic indexing", "err", err)
		return
	}

	slog.Info("embedder: starting background semantic indexing")

	failed := make(map[string]int) // path -> fail count; skip after 3 attempts
	for {
		paths, err := b.source.AllPaths(ctx)
		if err != nil {
			slog.Error("embedder: failed to get paths", "err", err)
			return
		}

		existing, err := b.vdb.AllVectors(ctx)
		if err != nil {
			slog.Error("embedder: failed to get existing vectors", "err", err)
			return
		}

		todo := make([]string, 0)
		for _, p := range paths {
			if _, ok := existing[p]; !ok && failed[p] < 3 {
				todo = append(todo, p)
			}
		}

		if len(todo) == 0 {
			slog.Info("embedder: all files embedded, sleeping", "total", len(existing))
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
			continue
		}

		slog.Info("embedder: files to embed", "count", len(todo), "already_done", len(existing))

		for i, path := range todo {
			select {
			case <-ctx.Done():
				slog.Info("embedder: stopped", "embedded", i)
				return
			default:
			}

			content, err := os.ReadFile(path)
			if err != nil {
				failed[path]++
				slog.Debug("embedder: skip unreadable file", "path", path, "attempt", failed[path], "err", err)
				time.Sleep(b.delay)
				continue
			}

			text := string(content)
			if len(text) == 0 {
				failed[path]++
				time.Sleep(b.delay)
				continue
			}

			vec, err := b.client.Embed(ctx, text)
			if err != nil {
				failed[path]++
				slog.Warn("embedder: embed failed", "path", path, "attempt", failed[path], "err", err)
				time.Sleep(b.delay)
				continue
			}

			if err := b.vdb.UpsertVector(ctx, path, vec); err != nil {
				slog.Error("embedder: store vector failed", "path", path, "err", err)
			} else {
				slog.Debug("embedder: embedded", "path", path, "progress", fmt.Sprintf("%d/%d", i+1, len(todo)))
			}

			time.Sleep(b.delay)
		}
	}
}
