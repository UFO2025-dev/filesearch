package db

import (
	"context"
	"os"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.db")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	f.Close()
	d, err := New(context.Background(), f.Name())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestUpsertAndSearch(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()

	if err := d.Upsert(ctx, "a.txt", "contrat achat fournisseur"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := d.Upsert(ctx, "b.txt", "facture mensuelle paiement"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	results, err := d.Search(ctx, "contrat", 10, 0, SearchFilter{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "a.txt" {
		t.Errorf("expected a.txt, got %s", results[0].Path)
	}
}

func TestUpsertIsAtomic(t *testing.T) {
	// Re-upserting the same path must not create a duplicate.
	d := newTestDB(t)
	ctx := context.Background()

	if err := d.Upsert(ctx, "x.txt", "original content here"); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if err := d.Upsert(ctx, "x.txt", "updated content here"); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	results, err := d.Search(ctx, "updated", 10, 0, SearchFilter{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected exactly 1 result after re-upsert, got %d", len(results))
	}
	results2, _ := d.Search(ctx, "original", 10, 0, SearchFilter{})
	if len(results2) != 0 {
		t.Errorf("old content still indexed: expected 0 results, got %d", len(results2))
	}
}

func TestDelete(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()

	d.Upsert(ctx, "del.txt", "document a supprimer absolument")
	if err := d.Delete(ctx, "del.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	results, _ := d.Search(ctx, "supprimer", 10, 0, SearchFilter{})
	if len(results) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(results))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	d := newTestDB(t)
	_, err := d.Search(context.Background(), "", 10, 0, SearchFilter{})
	if err == nil {
		t.Error("expected error on empty query, got nil")
	}
}

func TestSanitizeInjections(t *testing.T) {
	cases := []string{`"`, `NOT NOT`, `***`, `(((`, `^`}
	for _, q := range cases {
		_, err := sanitizeQuery(q)
		if err == nil {
			t.Errorf("sanitizeQuery(%q): expected error, got nil", q)
		}
	}
}

func TestSanitizeValidQuery(t *testing.T) {
	cases := map[string]string{
		"contrat":        "contrat",
		"  facture  ":   "facture",
		"achat NOT vente": "achat vente",
		"hello AND world": "hello world",
	}
	for input, want := range cases {
		got, err := sanitizeQuery(input)
		if err != nil {
			t.Errorf("sanitizeQuery(%q): unexpected error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("sanitizeQuery(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCount(t *testing.T) {
	d := newTestDB(t)
	ctx := context.Background()
	_ = d.Upsert(ctx, "/tmp/count1.txt", "hello world testing")
	_ = d.Upsert(ctx, "/tmp/count2.txt", "testing one two three")

	n, err := d.Count(ctx, "testing", SearchFilter{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Errorf("Count: got %d, want 2", n)
	}
}
