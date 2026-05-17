package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"gatewatch/file_search/internal/cache"
	"gatewatch/file_search/internal/db"
	"gatewatch/file_search/internal/indexer"
)

const batchDelay = 500 * time.Millisecond

// Watcher uses fsnotify to react to filesystem events with 500ms batching.
type Watcher struct {
	root     string
	database *db.DB
	cache    *cache.Cache
}

// New creates a Watcher for root.
func New(root string, database *db.DB, c *cache.Cache) *Watcher {
	return &Watcher{root: root, database: database, cache: c}
}

// Run starts the fsnotify event loop; returns when ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("watcher: fsnotify init failed", "err", err)
		return
	}
	defer fsw.Close()

	if err := addDirsRecursive(fsw, w.root); err != nil {
		slog.Error("watcher: failed to add dirs", "root", w.root, "err", err)
		return
	}
	slog.Info("watcher: watching", "root", w.root)

	// pending collects paths that need re-indexing; flushed after batchDelay.
	pending := make(map[string]bool) // path -> isDelete
	timer := time.NewTimer(batchDelay)
	timer.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("watcher: stopped")
			return

		case event, ok := <-fsw.Events:
			if !ok {
				return
			}
			path := filepath.Clean(event.Name)

			// Track new directories so deeply-nested creates are caught.
			if event.Has(fsnotify.Create) {
				if fi, err := os.Stat(path); err == nil && fi.IsDir() {
					_ = fsw.Add(path)
					continue
				}
			}

			if !isSupportedExt(path) {
				continue
			}

			isDelete := event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename)
			pending[path] = isDelete

			// Reset the debounce timer on every event.
			timer.Reset(batchDelay)

		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			slog.Warn("watcher: fsnotify error", "err", err)

		case <-timer.C:
			if len(pending) == 0 {
				continue
			}
			w.flush(ctx, pending)
			pending = make(map[string]bool)
		}
	}
}

const maxWatchedDirs = 10_000

// addDirsRecursive registers subdirectories of root with the watcher up to maxWatchedDirs.
// Capping prevents handle exhaustion on large trees (OneDrive, NAS, etc.).
func addDirsRecursive(fsw *fsnotify.Watcher, root string) error {
	count := 0
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if count >= maxWatchedDirs {
				slog.Warn("watcher: dir cap reached, subtree will not be watched",
					"cap", maxWatchedDirs, "path", path)
				return filepath.SkipDir
			}
			count++
			return fsw.Add(path)
		}
		return nil
	})
}

// flush processes all batched events.
func (w *Watcher) flush(ctx context.Context, pending map[string]bool) {
	var changed, deleted []string
	for path, isDelete := range pending {
		if isDelete {
			deleted = append(deleted, path)
		} else {
			changed = append(changed, path)
		}
	}

	for _, path := range deleted {
		slog.Info("watcher: deleted", "path", path)
		if err := w.database.Delete(ctx, path); err != nil {
			slog.Error("watcher: delete error", "path", path, "err", err)
		} else {
			w.cache.InvalidateByPath(path)
		}
	}

	if len(changed) > 0 {
		slog.Info("watcher: re-indexing changed files", "count", len(changed))
		for _, path := range changed {
			if err := indexer.IndexFile(ctx, w.database, path); err != nil {
				slog.Error("watcher: index error", "path", path, "err", err)
			} else {
				w.cache.InvalidateByPath(path)
			}
		}
	}
}

var supportedExts = map[string]bool{
	// Plain text
	".txt": true, ".md": true, ".html": true, ".htm": true,
	".csv": true, ".json": true, ".yaml": true, ".yml": true, ".rtf": true,
	// Documents
	".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
	".odt": true, ".ods": true, ".odp": true,
	// Code & config
	".xml": true, ".log": true, ".ini": true, ".cfg": true, ".conf": true,
	".toml": true, ".sh": true, ".bat": true, ".ps1": true,
	".py": true, ".js": true, ".ts": true, ".sql": true, ".tex": true,
	".go": true, ".rs": true, ".c": true, ".cpp": true, ".h": true,
	".java": true, ".rb": true, ".php": true,
	// Legacy Office
	".doc": true, ".xls": true, ".ppt": true,
}

func isSupportedExt(path string) bool {
	return supportedExts[strings.ToLower(filepath.Ext(path))]
}
