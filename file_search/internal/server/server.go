package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gatewatch/file_search/internal/db"
	"gatewatch/file_search/internal/embedder"
)

//go:embed static
var staticFiles embed.FS

// Server holds the HTTP server configuration and state.
type Server struct {
	addr           string
	hwMode         string
	limiter        *rateLimiterStore
	token          string
	db             *db.DB
	embedderClient *embedder.Client
	indexedRoots   []string
}

// New creates a new Server. database and emb may be nil for graceful degradation.
func New(addr, hwMode string, database *db.DB, emb *embedder.Client, roots []string, token string) *Server {
	return &Server{
		addr:           addr,
		hwMode:         hwMode,
		limiter:        newRateLimiterStore(),
		token:          token,
		db:             database,
		embedderClient: emb,
		indexedRoots:   roots,
	}
}

// chain wraps a handler with rate-limiting and auth middleware.
func (s *Server) chain(h http.Handler) http.Handler {
	return s.rateLimitMiddleware(s.authMiddleware(h))
}

// Run registers all routes and starts the HTTP server.
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// Static UI — no auth, no rate limit.
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("embed static: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// Health — no auth (used by Docker health checks etc.).
	mux.HandleFunc("/health", s.handleHealth)

	// Protected API routes.
	mux.Handle("GET /search", s.chain(http.HandlerFunc(s.handleSearch)))
	mux.Handle("POST /open", s.chain(http.HandlerFunc(s.handleOpen)))
	mux.Handle("GET /api/search/semantic", s.chain(http.HandlerFunc(s.handleSemanticSearch)))
	mux.Handle("GET /api/status", s.chain(http.HandlerFunc(s.handleStatus)))

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	slog.Info("listening", "addr", s.addr)
	return srv.ListenAndServe()
}

// handleHealth returns service liveness and readiness information.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	semanticReady := false
	if s.db != nil {
		if n, err := s.db.VectorCount(r.Context()); err == nil {
			semanticReady = n > 0
		}
	}
	writeJSON(w, map[string]any{
		"status":         "ok",
		"mode":           s.hwMode,
		"semantic_ready": semanticReady,
	})
}

// handleSearch runs a paginated FTS5 keyword search.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 10
	offset := (page - 1) * pageSize

	results := []db.Result{}
	total := 0

	if s.db != nil && q != "" {
		var err error
		results, err = s.db.Search(r.Context(), q, pageSize, offset)
		if err != nil {
			writeError(w, "search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		total, err = s.db.Count(r.Context(), q)
		if err != nil {
			writeError(w, "count failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if results == nil {
		results = []db.Result{}
	}

	pages := (total + pageSize - 1) / pageSize
	if pages < 1 {
		pages = 1
	}
	writeJSON(w, map[string]any{
		"results": results,
		"total":   total,
		"page":    page,
		"pages":   pages,
	})
}

// handleOpen converts a WSL path to a Windows path and opens it with Explorer.
func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Security: path must be under one of the indexed roots (if configured).
	if len(s.indexedRoots) > 0 {
		allowed := false
		for _, root := range s.indexedRoots {
			if root != "" && strings.HasPrefix(req.Path, root) {
				allowed = true
				break
			}
		}
		if !allowed {
			writeError(w, "path not in indexed roots", http.StatusForbidden)
			return
		}
	}

	out, err := exec.CommandContext(r.Context(), "wslpath", "-w", req.Path).Output()
	if err != nil {
		writeError(w, "wslpath: "+err.Error(), http.StatusInternalServerError)
		return
	}
	winPath := strings.TrimSpace(string(out))

	if err := exec.CommandContext(r.Context(), "cmd.exe", "/C", "start", "", winPath).Run(); err != nil {
		writeError(w, "start: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleSemanticSearch embeds the query and returns cosine-ranked results.
func (s *Server) handleSemanticSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, map[string]any{"results": []any{}})
		return
	}
	if s.embedderClient == nil || s.db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Service s\u00e9mantique non disponible"})
		return
	}

	queryVec, err := s.embedderClient.Embed(r.Context(), q)
	if err != nil {
		slog.Warn("semantic search: embed error", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Service s\u00e9mantique non disponible"})
		return
	}

	allVectors, err := s.db.AllVectors(r.Context())
	if err != nil {
		writeError(w, "vector load failed", http.StatusInternalServerError)
		return
	}

	type result struct {
		Path  string  `json:"path"`
		Score float32 `json:"score"`
	}
	hits := make([]result, 0)
	for path, vec := range allVectors {
		if isLibraryPath(path) {
			continue
		}
		score := embedder.CosineSimilarity(queryVec, vec)
		if score > 0.5 {
			hits = append(hits, result{Path: path, Score: score})
		}
	}
	// Sort descending by score.
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Score > hits[j-1].Score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	if len(hits) > 10 {
		hits = hits[:10]
	}
	writeJSON(w, map[string]any{"results": hits})
}

// handleStatus returns the semantic embedding progress.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var total, embedded int
	if s.db != nil {
		if paths, err := s.db.AllPaths(r.Context()); err == nil {
			total = len(paths)
		}
		embedded, _ = s.db.VectorCount(r.Context())
	}
	percent := 0
	if total > 0 {
		percent = (embedded * 100) / total
	}
	writeJSON(w, map[string]int{
		"embedded": embedded,
		"total":    total,
		"percent":  percent,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func isLibraryPath(path string) bool {
	for _, marker := range []string{"site-packages", "/env/Lib/", "/env/lib/", "node_modules", "vendor/", "/.git/"} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}
