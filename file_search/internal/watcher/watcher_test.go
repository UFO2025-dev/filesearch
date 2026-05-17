package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gatewatch/file_search/internal/cache"
	"gatewatch/file_search/internal/db"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "w-*.db")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	f.Close()
	d, err := db.New(context.Background(), f.Name())
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// TestWatcherCreateAndDelete verifies that the fsnotify-based watcher correctly
// indexes a new file and removes a deleted file from the DB.
// Uses a short timeout to let fsnotify deliver events.
func TestWatcherCreateAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fsnotify integration test in -short mode")
	}
	dir := t.TempDir()
	d := newTestDB(t)
	c := cache.New(10, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := New(dir, d, c)
	go w.Run(ctx)

	// Allow watcher to register dirs.
	time.Sleep(200 * time.Millisecond)

	// Create a file.
	p := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(p, []byte("contrat important document"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for batch flush (batchDelay + margin).
	time.Sleep(batchDelay + 600*time.Millisecond)

	results, err := d.Search(context.Background(), "contrat", 5, 0, db.SearchFilter{})
	if err != nil {
		t.Fatalf("Search after create: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected file to be indexed after creation")
	}

	// Delete the file.
	os.Remove(p)
	time.Sleep(batchDelay + 600*time.Millisecond)

	results2, _ := d.Search(context.Background(), "contrat", 5, 0, db.SearchFilter{})
	if len(results2) != 0 {
		t.Errorf("expected 0 results after deletion, got %d", len(results2))
	}
}

// TestWatcherFlush tests the flush logic directly without real filesystem events.
func TestWatcherFlush(t *testing.T) {
	dir := t.TempDir()
	d := newTestDB(t)
	c := cache.New(10, time.Minute)
	w := New(dir, d, c)

	// Upsert a file and then flush as deleted.
	p := filepath.Join(dir, "bye.txt")
	os.WriteFile(p, []byte("fichier a supprimer"), 0644)
	d.Upsert(context.Background(), p, "fichier a supprimer")

	w.flush(context.Background(), map[string]bool{p: true})

	results, _ := d.Search(context.Background(), "supprimer", 5, 0, db.SearchFilter{})
	if len(results) != 0 {
		t.Errorf("expected 0 after flush delete, got %d", len(results))
	}
}

func TestIsSupportedExt(t *testing.T) {
	cases := map[string]bool{
		"file.txt":  true,
		"file.md":   true,
		"file.pdf":  true,
		"file.html": true,
		"file.htm":  true,
		"file.jpg":  false,
		"file.go":   true,
		"file.exe":  false,
		"FILE.TXT":  true,
	}
	for name, want := range cases {
		got := isSupportedExt(name)
		if got != want {
			t.Errorf("isSupportedExt(%q) = %v, want %v", name, got, want)
		}
	}
}
