package server

import (
	"context"
	"embed"
	"encoding/json"
	"os"
	"sync"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os/exec"
	"strconv"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gatewatch/file_search/internal/cache"
	appcfg "gatewatch/file_search/internal/config"
	"gatewatch/file_search/internal/db"
	"gatewatch/file_search/internal/embedder"
)

//go:embed static
var staticFiles embed.FS

// Server holds the HTTP server configuration and state.
type Server struct {
	addr           string
	hwMode         string
	version        string
	limiter        *rateLimiterStore
	token          string
	db             *db.DB
	embedderClient *embedder.Client
	indexedRoots   []string
	cache          *cache.Cache
	dbPath          string
	mu             sync.Mutex
	modeOverride    string
	dirChangeCh     chan string
	cfgMgr          *appcfg.Manager
	httpSrv         *http.Server
}

// New creates a new Server. database and emb may be nil for graceful degradation.
func New(addr, hwMode, version string, database *db.DB, emb *embedder.Client, roots []string, token string, c *cache.Cache) *Server {
	return &Server{
		addr:           addr,
		hwMode:         hwMode,
		version:        version,
		limiter:        newRateLimiterStore(),
		token:          token,
		db:             database,
		embedderClient: emb,
		indexedRoots:   roots,
		cache:          c,
	}
}


// SetDBPath sets the path to the SQLite database file (used for stats).
func (s *Server) SetDBPath(path string) { s.dbPath = path }

// SetDirChangeCh sets the channel used to signal a new directory to index.
func (s *Server) SetDirChangeCh(ch chan string) { s.dirChangeCh = ch }

// SetConfig wires the persistent config manager so settings changes are saved.
func (s *Server) SetConfig(m *appcfg.Manager) { s.cfgMgr = m }

// effectiveMode returns the active mode (override takes precedence over detected).
func (s *Server) effectiveMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.modeOverride != "" {
		return s.modeOverride
	}
	return s.hwMode
}
// chain wraps a handler with panic recovery, CSRF, rate-limiting, and auth middleware.
func (s *Server) chain(h http.Handler) http.Handler {
	return s.recoveryMiddleware(s.csrfMiddleware(s.rateLimitMiddleware(s.authMiddleware(h))))
}

// Run registers all routes and starts the HTTP server.
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// Static UI â€” no auth, no rate limit.
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("embed static: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// Health â€” no auth (used by Docker health checks etc.).
	mux.HandleFunc("/health", s.handleHealth)

	// Protected API routes.
	mux.Handle("GET /search", s.chain(http.HandlerFunc(s.handleSearch)))
	mux.Handle("POST /open", s.chain(http.HandlerFunc(s.handleOpen)))
	mux.Handle("GET /api/search/semantic", s.chain(http.HandlerFunc(s.handleSemanticSearch)))
	mux.Handle("GET /api/status", s.chain(http.HandlerFunc(s.handleStatus)))
	mux.Handle("GET /api/config", s.chain(http.HandlerFunc(s.handleConfig)))
	mux.Handle("POST /api/settings", s.chain(http.HandlerFunc(s.handleSettings)))

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	s.mu.Lock()
	s.httpSrv = srv
	s.mu.Unlock()
	slog.Info("listening", "addr", s.addr)
	return srv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.limiter.Stop()
	s.mu.Lock()
	h := s.httpSrv
	s.mu.Unlock()
	if h == nil {
		return nil
	}
	return h.Shutdown(ctx)
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
		"app":            "filesearch",
		"version":        s.version,
		"status":         "ok",
		"mode":           s.effectiveMode(),
		"semantic_ready": semanticReady,
	})
}

// handleSearch runs a paginated FTS5 keyword search with optional filters.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 10
	offset := (page - 1) * pageSize

	f := db.SearchFilter{
		Ext:   r.URL.Query().Get("ext"),
		Since: r.URL.Query().Get("since"),
	}

	// Cache key includes all query dimensions.
	cacheKey := fmt.Sprintf("%s|%s|%s|%d", q, f.Ext, f.Since, page)

	results := []db.Result{}
	total := 0

	if s.db != nil && q != "" {
		// Check cache first.
		if s.cache != nil {
			if cached, cachedTotal, ok := s.cache.Get(cacheKey); ok {
				cacheResults := make([]db.Result, len(cached))
				for i, cr := range cached {
					cacheResults[i] = db.Result{Path: cr.Path, Snippet: cr.Snippet}
				}
				cachedPages := (cachedTotal + pageSize - 1) / pageSize
				if cachedPages < 1 {
					cachedPages = 1
				}
				writeJSON(w, map[string]any{
					"results": cacheResults,
					"total":   cachedTotal,
					"page":    page,
					"pages":   cachedPages,
					"cached":  true,
				})
				return
			}
		}

		var err error
		results, err = s.db.Search(r.Context(), q, pageSize, offset, f)
		if err != nil {
			writeError(w, "search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		total, err = s.db.Count(r.Context(), q, f)
		if err != nil {
			writeError(w, "count failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Store in cache.
		if s.cache != nil && len(results) > 0 {
			cacheItems := make([]cache.Result, len(results))
			for i, res := range results {
				cacheItems[i] = cache.Result{Path: res.Path, Snippet: res.Snippet}
			}
			s.cache.Set(cacheKey, cacheItems, total)
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
	slog.Info("handleOpen", "path", req.Path, "roots", s.indexedRoots)
	if len(s.indexedRoots) > 0 {
		allowed := false
		cleanPath := filepath.Clean(req.Path)
		for _, root := range s.indexedRoots {
			if root == "" {
				continue
			}
			cleanRoot := filepath.Clean(root)
			if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
				allowed = true
				break
			}
		}
		if !allowed {
			writeError(w, "path not in indexed roots: "+req.Path, http.StatusForbidden)
			return
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Use explorer.exe with the path as a direct argument — no shell involved,
		// so special chars (&, |, ;, etc.) in filenames cannot be interpreted.
		winPath := filepath.FromSlash(req.Path)
		slog.Info("handleOpen: opening", "winPath", winPath)
		cmd = exec.Command("explorer.exe", winPath)
	} else {
		out, err := exec.CommandContext(r.Context(), "wslpath", "-w", req.Path).Output()
		if err != nil {
			writeError(w, "wslpath: "+err.Error(), http.StatusInternalServerError)
			return
		}
		winPath := strings.TrimSpace(string(out))
		slog.Info("handleOpen: opening", "winPath", winPath)
		// Pass path directly to explorer — no cmd.exe shell interpretation.
		cmd = exec.Command("/mnt/c/Windows/System32/explorer.exe", winPath)
	}
	if err := cmd.Start(); err != nil {
		slog.Error("handleOpen: start failed", "err", err)
		writeError(w, "open: "+err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("handleOpen: success", "pid", cmd.Process.Pid)
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
		if !strings.HasPrefix(path, "/") {
			continue // skip relative paths stored by old indexer
		}
		if isLibraryPath(path) {
			continue
		}
		score := embedder.CosineSimilarity(queryVec, vec)
		if score > 0.5 {
			hits = append(hits, result{Path: path, Score: score})
		}
	}
	// Sort descending by score — O(n log n).
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > 10 {
		hits = hits[:10]
	}
	writeJSON(w, map[string]any{"results": hits})
}

// handleConfig returns server configuration and stats.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	embedded := 0
	total := 0
	if s.db != nil {
		embedded, _ = s.db.VectorCount(r.Context())
		total, _ = s.db.FileCount(r.Context())
	}
	var dbSizeBytes int64
	if s.dbPath != "" {
		if info, err := os.Stat(s.dbPath); err == nil {
			dbSizeBytes = info.Size()
		}
	}
	s.mu.Lock()
	modeOverride := s.modeOverride
	s.mu.Unlock()
	writeJSON(w, map[string]any{
		"mode":           s.effectiveMode(),
		"mode_override":  modeOverride,
		"indexed_roots":  s.indexedRoots,
		"files_indexed":  total,
		"files_embedded": embedded,
		"db_size_bytes":  dbSizeBytes,
	})
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

// handleSettings updates server configuration at runtime.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dir          string `json:"dir"`
		ModeOverride string `json:"mode_override"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid body", http.StatusBadRequest)
		return
	}
	// Validate directory before acquiring the lock.
	if req.Dir != "" {
		clean := filepath.Clean(req.Dir)
		if !filepath.IsAbs(clean) {
			writeError(w, "dir must be an absolute path", http.StatusBadRequest)
			return
		}
		info, err := os.Stat(clean)
		if err != nil {
			writeError(w, "directory does not exist or is not accessible", http.StatusBadRequest)
			return
		}
		if !info.IsDir() {
			writeError(w, "path is not a directory", http.StatusBadRequest)
			return
		}
		if isSensitivePath(clean) {
			writeError(w, "cannot index system or sensitive directory", http.StatusForbidden)
			return
		}
		req.Dir = clean
	}
	s.mu.Lock()
	if req.ModeOverride == "auto" {
		s.modeOverride = ""
	} else if req.ModeOverride != "" {
		s.modeOverride = req.ModeOverride
	}
	if req.Dir != "" {
		found := false
		for _, root := range s.indexedRoots {
			if root == req.Dir {
				found = true
				break
			}
		}
		if !found {
			s.indexedRoots = append(s.indexedRoots, req.Dir)
		}
	}
	s.mu.Unlock()

	// Persist changes to disk.
	if s.cfgMgr != nil {
		if req.Dir != "" {
			_ = s.cfgMgr.AddDir(req.Dir)
		}
		if req.ModeOverride != "" {
			mode := req.ModeOverride
			if mode == "auto" {
				mode = ""
			}
			_ = s.cfgMgr.SetModeOverride(mode)
		}
	}

	if req.Dir != "" && s.dirChangeCh != nil {
		select {
		case s.dirChangeCh <- req.Dir:
		default:
		}
	}
	writeJSON(w, map[string]bool{"ok": true})
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

// isSensitivePath returns true for known system/OS directories that should not be indexed.
func isSensitivePath(p string) bool {
	lower := strings.ToLower(filepath.ToSlash(p))
	prefixes := []string{
		// Windows
		"c:/windows", "c:/program files", "c:/program files (x86)",
		"c:/programdata", "c:/system volume information",
		// Linux / macOS
		"/proc", "/sys", "/dev", "/run", "/boot",
	}
	for _, prefix := range prefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+"/") {
			return true
		}
	}
	return false
}

