//go:build ignore

// generate.go — synthetic French document corpus generator.
// Usage: go run ./benchmarks/datasets/generate.go -n 10000 -dir /tmp/bench-corpus
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

var frenchWords = []string{
	"contrat", "avocat", "facture", "client", "dossier", "rapport",
	"document", "accord", "tribunal", "jugement", "appel", "expertise",
	"contenu", "article", "clause", "annexe", "protocole", "convention",
	"bilan", "resultat", "budget", "depense", "recette", "tresorerie",
	"patient", "medecin", "ordonnance", "diagnostic", "traitement",
	"prescription", "analyse", "examen", "consultation", "dossier",
	"recherche", "fichier", "archive", "sauvegarde", "backup",
	"contrat-cadre", "sous-traitant", "partenaire", "litige",
}

var exts = []struct {
	ext     string
	weight  int
}{
	{"txt", 50},
	{"md", 20},
	{"html", 20},
	{"csv", 5},
	{"json", 5},
}

func main() {
	n := flag.Int("n", 1000, "number of files to generate")
	dir := flag.String("dir", "/tmp/bench-corpus", "output directory")
	seed := flag.Int64("seed", 42, "random seed (for reproducibility)")
	flag.Parse()

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	r := rand.New(rand.NewSource(*seed))
	for i := 0; i < *n; i++ {
		ext := pickExt(r)
		path := filepath.Join(*dir, fmt.Sprintf("doc_%06d.%s", i, ext))
		content := generateContent(r, ext)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
		if i%1000 == 0 {
			fmt.Printf("  %d / %d files generated...\n", i, *n)
		}
	}
	fmt.Printf("Done: %d files in %s\n", *n, *dir)
}

func pickExt(r *rand.Rand) string {
	total := 0
	for _, e := range exts {
		total += e.weight
	}
	v := r.Intn(total)
	for _, e := range exts {
		v -= e.weight
		if v < 0 {
			return e.ext
		}
	}
	return "txt"
}

func sentence(r *rand.Rand, n int) string {
	words := make([]byte, 0, n*10)
	for i := 0; i < n; i++ {
		if i > 0 {
			words = append(words, ' ')
		}
		words = append(words, frenchWords[r.Intn(len(frenchWords))]...)
	}
	return string(words)
}

func generateContent(r *rand.Rand, ext string) string {
	switch ext {
	case "md":
		return fmt.Sprintf("# %s\n\n%s\n\n## Details\n\n%s\n",
			sentence(r, 5), sentence(r, 30), sentence(r, 20))
	case "html":
		return fmt.Sprintf("<html><body><h1>%s</h1><p>%s</p><p>%s</p></body></html>",
			sentence(r, 5), sentence(r, 30), sentence(r, 15))
	case "csv":
		return fmt.Sprintf("id,titre,contenu\n1,%s,%s\n2,%s,%s\n",
			sentence(r, 3), sentence(r, 10), sentence(r, 3), sentence(r, 10))
	case "json":
		return fmt.Sprintf(`{"title":"%s","content":"%s","tags":["%s","%s"]}`,
			sentence(r, 4), sentence(r, 20),
			frenchWords[r.Intn(len(frenchWords))],
			frenchWords[r.Intn(len(frenchWords))])
	default:
		return sentence(r, 40+r.Intn(40))
	}
}
