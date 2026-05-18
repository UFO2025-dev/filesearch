package benchmarks_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gatewatch/file_search/internal/db"
)

// ── French corpus ─────────────────────────────────────────────────────────────

var frenchWords = []string{
	"contrat", "avocat", "facture", "client", "dossier", "rapport",
	"document", "accord", "tribunal", "jugement", "appel", "expertise",
	"contenu", "article", "clause", "annexe", "protocole", "convention",
	"bilan", "resultat", "budget", "depense", "recette", "tresorerie",
	"patient", "medecin", "ordonnance", "diagnostic", "traitement",
	"prescription", "analyse", "examen", "consultation",
	"recherche", "fichier", "archive", "sauvegarde", "backup",
}

func makeSentence(r *rand.Rand, words int) string {
	var b strings.Builder
	for i := 0; i < words; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(frenchWords[r.Intn(len(frenchWords))])
	}
	return b.String()
}

type fileEntry struct{ path, content string }

func makeCorpus(t testing.TB, dir string, n int) []fileEntry {
	t.Helper()
	r := rand.New(rand.NewSource(42))
	entries := make([]fileEntry, n)
	for i := 0; i < n; i++ {
		content := makeSentence(r, 20+r.Intn(30))
		path := filepath.Join(dir, fmt.Sprintf("doc_%04d.txt", i))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		entries[i] = fileEntry{path: path, content: content}
	}
	return entries
}

// ── DB helper ─────────────────────────────────────────────────────────────────

// newBenchDB opens a temp DB pre-populated with n rows.
// Caller must call b.ResetTimer() after this returns.
func newBenchDB(b *testing.B, n int) *db.DB {
	b.Helper()
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	d, err := db.New(context.Background(), dbPath)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = d.Close() })
	r := rand.New(rand.NewSource(99))
	for i := 0; i < n; i++ {
		content := makeSentence(r, 20+r.Intn(30))
		path := fmt.Sprintf("/bench/doc_%04d.txt", i)
		if err := d.Upsert(context.Background(), path, content); err != nil {
			b.Fatal(err)
		}
	}
	return d
}

// ── Percentile helpers ────────────────────────────────────────────────────────

func pct(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[(len(sorted)-1)*p/100]
}

func reportPercentiles(b *testing.B, samples []int64) {
	b.Helper()
	if len(samples) == 0 {
		return
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	b.ReportMetric(float64(pct(samples, 50)), "ns/p50")
	b.ReportMetric(float64(pct(samples, 95)), "ns/p95")
	b.ReportMetric(float64(pct(samples, 99)), "ns/p99")
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
