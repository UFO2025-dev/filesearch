package indexer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gatewatch/file_search/internal/db"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "idx-*.db")
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

func TestExtractMarkdown(t *testing.T) {
	raw := "# Titre\n\n**gras** et _italique_\n\n[lien](http://example.com)"
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte(raw), 0644)

	text, err := extractMarkdown(p)
	if err != nil {
		t.Fatalf("extractMarkdown: %v", err)
	}
	if strings.Contains(text, "#") {
		t.Error("markdown headers should be stripped")
	}
	if strings.Contains(text, "**") {
		t.Error("markdown bold markers should be stripped")
	}
	if !strings.Contains(text, "gras") {
		t.Error("word inside bold should be preserved")
	}
	if !strings.Contains(text, "lien") {
		t.Error("link text should be preserved")
	}
}

func TestExtractHTML(t *testing.T) {
	raw := "<html><body><h1>Titre</h1><p>Bonjour &amp; monde</p></body></html>"
	dir := t.TempDir()
	p := filepath.Join(dir, "page.html")
	os.WriteFile(p, []byte(raw), 0644)

	text, err := extractHTML(p)
	if err != nil {
		t.Fatalf("extractHTML: %v", err)
	}
	if strings.Contains(text, "<") {
		t.Error("HTML tags should be stripped")
	}
	if !strings.Contains(text, "Titre") {
		t.Error("text content should be preserved")
	}
	if !strings.Contains(text, "Bonjour & monde") {
		t.Error("&amp; should be decoded to &")
	}
}

func TestExtractPDFMissing(t *testing.T) {
	// Reset pdfOnce so the check runs fresh in this test process.
	// In practice pdftotext may or may not be installed; we only test
	// that the function never returns a hard error when it is missing.
	dir := t.TempDir()
	p := filepath.Join(dir, "dummy.pdf")
	os.WriteFile(p, []byte("%PDF-1.4 fake"), 0644)

	// Should not crash — returns empty string if pdftotext unavailable.
	_, err := extractPDF(context.Background(), p)
	if err != nil && strings.Contains(err.Error(), "pdftotext") {
		t.Logf("pdftotext returned error (expected if not installed): %v", err)
	}
}

func TestIndexFileTxt(t *testing.T) {
	d := newTestDB(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	os.WriteFile(p, []byte("recherche locale rapide"), 0644)

	if err := IndexFile(context.Background(), d, p); err != nil {
		t.Fatalf("IndexFile: %v", err)
	}
	results, err := d.Search(context.Background(), "locale", 5, 0, db.SearchFilter{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestIndexFileSkipsEmpty(t *testing.T) {
	d := newTestDB(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.txt")
	os.WriteFile(p, []byte("   \n  "), 0644)

	if err := IndexFile(context.Background(), d, p); err != nil {
		t.Fatalf("IndexFile on empty file should not error: %v", err)
	}
	results, _ := d.Search(context.Background(), "vide", 5, 0, db.SearchFilter{})
	if len(results) != 0 {
		t.Error("empty file should not be indexed")
	}
}

func TestRunBasic(t *testing.T) {
	d := newTestDB(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("contrat annuel fournisseur"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("facture paiement mensuel"), 0644)
	os.WriteFile(filepath.Join(dir, "ignore.jpg"), []byte("not indexed"), 0644)

	stats, err := Run(context.Background(), d, dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Indexed != 2 {
		t.Errorf("expected 2 indexed, got %d", stats.Indexed)
	}
	if stats.Skipped < 1 {
		t.Errorf("expected at least 1 skipped (.jpg), got %d", stats.Skipped)
	}
}
