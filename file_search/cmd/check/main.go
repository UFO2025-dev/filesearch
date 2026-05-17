package main

import (
    "context"
    "fmt"
    "log"

    "gatewatch/file_search/internal/db"
)

func main() {
    ctx := context.Background()
    d, err := db.New(ctx, "data/index.db")
    if err != nil { log.Fatal(err) }
    defer d.Close()

    queries := []string{"contrat", "facture", "performance", "dinars"}
    for _, q := range queries {
        results, err := d.Search(ctx, q, 5, 0, db.SearchFilter{})
        if err != nil { log.Printf("search %q: %v", q, err); continue }
        fmt.Printf("Search %q -> %d results\n", q, len(results))
        for _, r := range results {
            fmt.Printf("  %s | %s\n", r.Path, r.Snippet)
        }
    }
}
