package interfaces

import "context"

// Searcher is the read interface for the FTS index.
type Searcher interface {
	Search(ctx context.Context, query string, limit, offset int) ([]SearchResult, error)
}

// SearchResult is the shared result type across the application.
type SearchResult struct {
	Path    string
	Snippet string
}

// Indexer indexes a single file into the search index.
type Indexer interface {
	IndexFile(ctx context.Context, path string) error
	Delete(ctx context.Context, path string) error
}

// Cacher is the cache interface used by the server and watcher.
type Cacher interface {
	Get(key string) ([]CacheResult, bool)
	Set(key string, results []CacheResult)
	Flush()
}

// CacheResult is the cache-layer result type.
type CacheResult struct {
	Path    string
	Snippet string
}
