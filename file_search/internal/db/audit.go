package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AuditEntry is one row from the audit_log table.
type AuditEntry struct {
	ID         int64  `json:"id"`
	SearchedAt string `json:"searched_at"`
	Query      string `json:"query"`
	Mode       string `json:"mode"`        // "classic" | "semantic"
	ResultsN   int    `json:"results_n"`
	DurationMs int64  `json:"duration_ms"`
}

// EnsureAuditTable creates the audit_log table if it doesn't exist.
func (d *DB) EnsureAuditTable(ctx context.Context) error {
	_, err := d.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit_log (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			searched_at TEXT    NOT NULL,
			query       TEXT    NOT NULL,
			mode        TEXT    NOT NULL DEFAULT 'classic',
			results_n   INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0
		)`)
	return err
}

// LogSearch appends one entry to audit_log (non-blocking — called from a goroutine).
func (d *DB) LogSearch(ctx context.Context, query, mode string, resultsN int, durationMs int64) error {
	_, err := d.conn.ExecContext(ctx,
		`INSERT INTO audit_log (searched_at, query, mode, results_n, duration_ms)
		 VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339),
		query, mode, resultsN, durationMs,
	)
	return err
}

// AuditLog returns the most recent entries (up to limit).
func (d *DB) AuditLog(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, searched_at, query, mode, results_n, duration_ms
		 FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.SearchedAt, &e.Query, &e.Mode, &e.ResultsN, &e.DurationMs); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// AuditCSV returns all audit entries as a CSV string (UTF-8 with BOM for Excel).
func (d *DB) AuditCSV(ctx context.Context) (string, error) {
	entries, err := d.AuditLog(ctx, 1000)
	if err != nil {
		return "", fmt.Errorf("audit csv: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM — makes Excel open accented chars correctly
	sb.WriteString("ID,Date,Requete,Mode,Resultats,Duree_ms\n")
	for _, e := range entries {
		q := strings.ReplaceAll(e.Query, `"`, `""`)
		sb.WriteString(fmt.Sprintf(`%d,"%s","%s","%s",%d,%d`+"\n",
			e.ID, e.SearchedAt, q, e.Mode, e.ResultsN, e.DurationMs))
	}
	return sb.String(), nil
}
