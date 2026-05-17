package db

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// EnsureVectorTable creates the embeddings table if it doesn't exist.
func (d *DB) EnsureVectorTable(ctx context.Context) error {
	_, err := d.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS embeddings (
			path       TEXT PRIMARY KEY,
			vector     BLOB NOT NULL,
			indexed_at TEXT NOT NULL
		)`)
	return err
}

// UpsertVector stores or updates the embedding vector for a file path.
func (d *DB) UpsertVector(ctx context.Context, path string, vec []float32) error {
	blob := float32SliceToBytes(vec)
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO embeddings (path, vector, indexed_at)
		VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET vector=excluded.vector, indexed_at=excluded.indexed_at`,
		path, blob, time.Now().UTC().Format(time.RFC3339))
	return err
}

// DeleteVector removes the embedding for a file path.
func (d *DB) DeleteVector(ctx context.Context, path string) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM embeddings WHERE path = ?`, path)
	return err
}

// AllVectors loads all stored embeddings into memory for similarity search.
func (d *DB) AllVectors(ctx context.Context) (map[string][]float32, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT path, vector FROM embeddings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]float32)
	for rows.Next() {
		var path string
		var blob []byte
		if err := rows.Scan(&path, &blob); err != nil {
			return nil, err
		}
		vec, err := bytesToFloat32Slice(blob)
		if err != nil {
			continue
		}
		result[path] = vec
	}
	return result, rows.Err()
}

// VectorCount returns the number of indexed embeddings.
func (d *DB) VectorCount(ctx context.Context) (int, error) {
	var n int
	err := d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM embeddings`).Scan(&n)
	return n, err
}

func float32SliceToBytes(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func bytesToFloat32Slice(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("invalid vector blob length %d", len(buf))
	}
	vec := make([]float32, len(buf)/4)
	for i := range vec {
		bits := binary.LittleEndian.Uint32(buf[i*4:])
		vec[i] = math.Float32frombits(bits)
	}
	return vec, nil
}
