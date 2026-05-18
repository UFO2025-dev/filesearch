package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

// ErrInvalidQuery is returned when the search query is empty or contains only special characters.
var ErrInvalidQuery = errors.New("invalid query")

// Result represents a single search result.
type Result struct {
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
}

// DB wraps the SQLite connection.
type DB struct {
	conn *sql.DB
}

// New opens (or creates) the SQLite database at path and initialises the schema.
func New(ctx context.Context, path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	conn.SetMaxOpenConns(1)

	d := &DB{conn: conn}
	if err := d.applyPragmas(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	if err := d.migrate(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) applyPragmas(ctx context.Context) error {
	pragmas := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA cache_size=-32000`,
		`PRAGMA temp_store=MEMORY`,
		`PRAGMA mmap_size=268435456`,
	}
	for _, p := range pragmas {
		if _, err := d.conn.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return nil
}

// currentSchemaVersion is the expected PRAGMA user_version for this build.
// Bump this constant whenever a new migration step is added.
const currentSchemaVersion = 3

func (d *DB) migrate(ctx context.Context) error {
	// Read the current schema version from SQLite's built-in user_version pragma.
	var version int
	row := d.conn.QueryRowContext(ctx, `PRAGMA user_version`)
	if err := row.Scan(&version); err != nil {
		return fmt.Errorf("db migrate: cannot read user_version: %w", err)
	}

	// v0 → v1: create FTS5 documents + files shadow table.
	if version < 1 {
		_, err := d.conn.ExecContext(ctx, `
			CREATE VIRTUAL TABLE IF NOT EXISTS documents USING fts5(
				path     UNINDEXED,
				content,
				tokenize = 'porter'
			)
		`)
		if err != nil {
			return fmt.Errorf("db migrate v1 fts5: %w", err)
		}
		_, err = d.conn.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS files (
				path      TEXT    PRIMARY KEY,
				doc_rowid INTEGER NOT NULL,
				hash      TEXT    NOT NULL DEFAULT '',
				mtime     INTEGER NOT NULL DEFAULT 0
			)
		`)
		if err != nil {
			return fmt.Errorf("db migrate v1 files: %w", err)
		}
		// Backfill from existing documents (handles upgrade from pre-files schema).
		_, err = d.conn.ExecContext(ctx, `
			INSERT OR IGNORE INTO files(path, doc_rowid, hash)
			SELECT path, rowid, '' FROM documents
		`)
		if err != nil {
			return fmt.Errorf("db migrate v1 backfill: %w", err)
		}
	}

	// v1 → v2: add mtime column to files (idempotent — ignore error if already present).
	if version < 2 {
		_, _ = d.conn.ExecContext(ctx, `ALTER TABLE files ADD COLUMN mtime INTEGER NOT NULL DEFAULT 0`)
	}

	// v2 → v3: create audit_log table.
	if version < 3 {
		if err := d.EnsureAuditTable(ctx); err != nil {
			return fmt.Errorf("db migrate v3 audit: %w", err)
		}
	}

	// Stamp the new version only if we advanced.
	if version < currentSchemaVersion {
		if _, err := d.conn.ExecContext(ctx,
			fmt.Sprintf(`PRAGMA user_version = %d`, currentSchemaVersion)); err != nil {
			return fmt.Errorf("db migrate: cannot set user_version: %w", err)
		}
	}
	return nil
}

// contentHash returns the SHA256 hex digest of content.
func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// Upsert inserts or replaces a document atomically.
// If the content hash matches the stored hash the operation is a no-op (O(1)).
// Delete + Insert use the stored rowid for O(1) FTS5 access.
func (d *DB) UpsertWithMtime(ctx context.Context, path, content string, mtime int64) error {
	hash := contentHash(content)

	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db upsert begin: %w", err)
	}
	defer tx.Rollback()

	var existingRowid int64
	var existingHash string
	err = tx.QueryRowContext(ctx,
		`SELECT doc_rowid, hash FROM files WHERE path = ?`, path).
		Scan(&existingRowid, &existingHash)

	switch {
	case err == nil && existingHash == hash:
		// Content unchanged — nothing to do.
		return tx.Commit()

	case err == nil:
		// Path exists but content changed: delete old FTS5 row by rowid (O(1)).
		if _, err = tx.ExecContext(ctx,
			`DELETE FROM documents WHERE rowid = ?`, existingRowid); err != nil {
			return fmt.Errorf("db upsert delete doc: %w", err)
		}

	case err == sql.ErrNoRows:
		// New path — nothing to delete.

	default:
		return fmt.Errorf("db upsert lookup: %w", err)
	}

	// Insert new FTS5 row and capture its rowid.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO documents(path, content) VALUES (?, ?)`, path, content)
	if err != nil {
		return fmt.Errorf("db upsert insert: %w", err)
	}
	newRowid, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("db upsert rowid: %w", err)
	}

	// Upsert the files shadow row.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO files(path, doc_rowid, hash, mtime) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET doc_rowid = excluded.doc_rowid, hash = excluded.hash, mtime = excluded.mtime`,
		path, newRowid, hash, mtime)
	if err != nil {
		return fmt.Errorf("db upsert files: %w", err)
	}
	return tx.Commit()
}

// Upsert indexes path with content, using mtime=0 (backward-compat shim).
func (d *DB) Upsert(ctx context.Context, path, content string) error {
	return d.UpsertWithMtime(ctx, path, content, 0)
}

// Delete removes a document from the index by path using O(1) rowid lookup.
func (d *DB) Delete(ctx context.Context, path string) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db delete begin: %w", err)
	}
	defer tx.Rollback()

	var rowid int64
	err = tx.QueryRowContext(ctx,
		`SELECT doc_rowid FROM files WHERE path = ?`, path).Scan(&rowid)
	if err == sql.ErrNoRows {
		return tx.Commit() // already gone
	}
	if err != nil {
		return fmt.Errorf("db delete lookup: %w", err)
	}

	if _, err = tx.ExecContext(ctx,
		`DELETE FROM documents WHERE rowid = ?`, rowid); err != nil {
		return fmt.Errorf("db delete doc: %w", err)
	}
	if _, err = tx.ExecContext(ctx,
		`DELETE FROM files WHERE path = ?`, path); err != nil {
		return fmt.Errorf("db delete files: %w", err)
	}
	return tx.Commit()
}

var (
	fts5SpecialRe  = regexp.MustCompile(`["^*(){}\[\]\\:;,]`)
	fts5KeywordsRe = regexp.MustCompile(`(?i)\b(NOT|AND|OR)\b`)
)

// sanitizeQuery cleans user input for safe use in FTS5 MATCH.
func sanitizeQuery(q string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidQuery)
	}
	q = fts5SpecialRe.ReplaceAllString(q, " ")
	q = fts5KeywordsRe.ReplaceAllString(q, " ")
	q = strings.Join(strings.Fields(q), " ")
	if q == "" {
		return "", fmt.Errorf("%w: only special characters or operators", ErrInvalidQuery)
	}
	return q, nil
}

// Search performs a full-text search and returns up to limit results starting at offset.
// Use offset = (page-1)*limit for pagination.
// SearchFilter holds optional filters for Search and Count.
type SearchFilter struct {
	Ext   string // e.g. ".pdf" — empty means all
	Since string // "today", "week", "month" — empty means all
}

func (d *DB) Search(ctx context.Context, query string, limit, offset int, f SearchFilter) ([]Result, error) {
	safe, err := sanitizeQuery(query)
	if err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	extClause, sinceClause, extParam := filterClauses(f)
	q := `SELECT path, snippet(documents, 1, '[', ']', '...', 15)
		 FROM documents
		 WHERE documents MATCH ?` + extClause + sinceClause + `
		 ORDER BY rank
		 LIMIT ? OFFSET ?`
	var qArgs []any
	qArgs = append(qArgs, safe)
	if extParam != "" {
		qArgs = append(qArgs, extParam)
	}
	qArgs = append(qArgs, limit, offset)
	rows, err := d.conn.QueryContext(ctx, q, qArgs...)
	if err != nil {
		return nil, fmt.Errorf("db search: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Path, &r.Snippet); err != nil {
			return nil, fmt.Errorf("db scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Count returns the total number of documents matching query (for pagination).
func (d *DB) Count(ctx context.Context, query string, f SearchFilter) (int, error) {
	safe, err := sanitizeQuery(query)
	if err != nil {
		return 0, fmt.Errorf("invalid query: %w", err)
	}
	extClause, sinceClause, extParam := filterClauses(f)
	var n int
	var cArgs []any
	cArgs = append(cArgs, safe)
	if extParam != "" {
		cArgs = append(cArgs, extParam)
	}
	err = d.conn.QueryRowContext(ctx,
		`SELECT count(*) FROM documents WHERE documents MATCH ?`+extClause+sinceClause, cArgs...,
	).Scan(&n)
	return n, err
}

// filterClauses returns SQL AND clauses for ext and date filters.
func filterClauses(f SearchFilter) (extClause, sinceClause, extParam string) {
	if f.Ext != "" {
		extClause = " AND path LIKE ?"
		extParam = "%" + f.Ext
	}
	switch f.Since {
	case "today":
		sinceClause = " AND documents.path IN (SELECT path FROM files WHERE mtime >= strftime('%s', 'now', '-1 day'))"
	case "week":
		sinceClause = " AND documents.path IN (SELECT path FROM files WHERE mtime >= strftime('%s', 'now', '-7 days'))"
	case "month":
		sinceClause = " AND documents.path IN (SELECT path FROM files WHERE mtime >= strftime('%s', 'now', '-30 days'))"
	}
	return
}

// Optimize runs FTS5 OPTIMIZE to merge index segments (call periodically).
func (d *DB) Optimize(ctx context.Context) error {
	_, err := d.conn.ExecContext(ctx, `INSERT INTO documents(documents) VALUES('optimize')`)
	return err
}

// Ping verifies the database connection is alive with a lightweight query.
func (d *DB) Ping(ctx context.Context) error {
	var n int
	return d.conn.QueryRowContext(ctx, `SELECT 1`).Scan(&n)
}

// Close closes the underlying connection.
// Checkpoint flushes the WAL to the main database file, reducing corruption risk on hard kills.
func (d *DB) Checkpoint() {
	_, _ = d.conn.ExecContext(context.Background(), `PRAGMA wal_checkpoint(TRUNCATE)`)
}
// IntegrityCheck runs PRAGMA integrity_check and returns an error if the DB is corrupt.
func (d *DB) IntegrityCheck(ctx context.Context) error {
	row := d.conn.QueryRowContext(ctx, `PRAGMA integrity_check`)
	var result string
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("integrity_check query: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("database integrity check failed: %s", result)
	}
	return nil
}



func (d *DB) Close() error {
	return d.conn.Close()
}


// AllPaths returns all file paths currently in the FTS5 index.
func (d *DB) AllPaths(ctx context.Context) ([]string, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT path FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// FileCount returns the total number of indexed files without loading all paths.
func (d *DB) FileCount(ctx context.Context) (int, error) {
	var n int
	err := d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM files`).Scan(&n)
	return n, err
}

