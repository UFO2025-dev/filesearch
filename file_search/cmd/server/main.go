package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"path/filepath"
	"strings"
	"time"

	"gatewatch/file_search/internal/cache"
	appcfg "gatewatch/file_search/internal/config"
	"gatewatch/file_search/internal/db"
	"gatewatch/file_search/internal/embedder"
	"gatewatch/file_search/internal/hardware"
	"gatewatch/file_search/internal/indexer"
	"gatewatch/file_search/internal/logger"
	"gatewatch/file_search/internal/server"
	"gatewatch/file_search/internal/watcher"
)

// Version is injected at build time via:
//   -ldflags="-X main.Version=1.0.1"
// Falls back to "dev" when built without the flag.
var Version = "dev"


// waitForServer polls /health until the server responds or timeout is reached.
// Returns true if server is ready.
func waitForServer(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// isFileSearchRunning checks if a FileSearch instance is already on the port.
// Returns true only if /health responds with {"app":"filesearch",...} to avoid
// false positives when another service occupies the port.
func isFileSearchRunning(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	return strings.Contains(string(body), `"app":"filesearch"`) ||
		strings.Contains(string(body), `"app": "filesearch"`)
}

// openBrowser opens the default browser at the given URL.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd.exe", "/C", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// showFatalError shows an error to the user even when compiled with -H windowsgui
// (no console). On Windows it spawns a MessageBox via PowerShell. On other
// platforms it writes to stderr (console is available there).
func showFatalError(title, msg string) {
	if runtime.GOOS == "windows" {
		script := fmt.Sprintf(
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`[System.Windows.Forms.MessageBox]::Show('%s', '%s', 0, 16) | Out-Null`,
			strings.ReplaceAll(msg, "'", "''"),
			strings.ReplaceAll(title, "'", "''"),
		)
		_ = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Run()
	} else {
		fmt.Fprintf(os.Stderr, "[FileSearch] %s: %s\n", title, msg)
	}
}

func main() {
	jsonLog := flag.Bool("json-log", false, "output logs as JSON")
	// FIX 3: bind 127.0.0.1 by default — reduces firewall friction + attack surface
	addr    := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	port    := flag.Int("port", 0, "listen port (overrides -addr)")
	dir     := flag.String("dir", "", "directory to index (added to config)")
	dbPath  := flag.String("db", "data/index.db", "SQLite database path")
	cfgPath := flag.String("config", "data/config.json", "config file path")
	token   := flag.String("token", "", "optional Bearer auth token")
	noOpen  := flag.Bool("no-browser", false, "do not open browser on startup")
	flag.Parse()

	// Resolve relative DB/config paths relative to the executable directory.
	// This prevents "data/" being created in the user's CWD on double-click.
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if !filepath.IsAbs(*dbPath) {
			abs := filepath.Join(exeDir, *dbPath)
			dbPath = &abs
		}
		if !filepath.IsAbs(*cfgPath) {
			abs := filepath.Join(exeDir, *cfgPath)
			cfgPath = &abs
		}
	}

	logger.Init(*jsonLog)

	if *port != 0 {
		*addr = fmt.Sprintf("127.0.0.1:%d", *port)
	}

	// FIX 2: Safe port collision — check if FileSearch already running
	baseURL := "http://" + *addr
	if isFileSearchRunning(baseURL) {
		slog.Info("FileSearch already running, opening browser", "url", baseURL)
		if !*noOpen {
			openBrowser(baseURL)
		}
		return
	}
	// Check port occupied by something else
	checkConn, err := http.Get(baseURL + "/health")
	if err == nil {
		checkConn.Body.Close()
		msg := fmt.Sprintf("Port %s is occupied by another application.\nStop that app or use -port to choose a different port.", *addr)
		slog.Error("port occupied by another application — cannot start", "addr", *addr)
		showFatalError("FileSearch — Cannot Start", msg)
		os.Exit(1)
	}

	hwProfile := hardware.Detect()
	hwProfile.Log()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		slog.Error("failed to create data dir", "err", err)
		os.Exit(1)
	}

	// ── Load persistent config ────────────────────────────────────────────
	cfg, err := appcfg.Load(*cfgPath)
	if err != nil {
		slog.Warn("config: failed to load, using defaults", "err", err)
		cfg, _ = appcfg.Load("")
	}

	if *dir != "" {
		if err := cfg.AddDir(*dir); err != nil {
			slog.Warn("config: failed to save new dir", "err", err)
		}
	}

	snapshot := cfg.Get()
	roots := snapshot.IndexedDirs

	// ── Database ──────────────────────────────────────────────────────────
	database, err := db.New(ctx, *dbPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.EnsureVectorTable(ctx); err != nil {
		slog.Warn("embedder: failed to create vector table", "err", err)
	}

	// Integrity check at startup — warn but don't exit (user might still search).
	if err := database.IntegrityCheck(ctx); err != nil {
		slog.Error("database integrity check failed — DB may be corrupt", "err", err)
	} else {
		slog.Info("database integrity check passed")
	}

	// Periodic FTS5 OPTIMIZE every 24 hours.
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := database.Optimize(context.Background()); err != nil {
					slog.Warn("fts5 optimize failed", "err", err)
				} else {
					slog.Info("fts5 optimize completed")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	embClient := embedder.New("", "")
	searchCache := cache.New(128, 30*time.Second)

	// ── Index all configured directories ─────────────────────────────────
	for _, d := range roots {
		d := d
		go func() {
			slog.Info("indexer: starting", "dir", d)
			stats, err := indexer.Run(ctx, database, d)
			if err != nil {
				slog.Error("indexer: failed", "dir", d, "err", err)
				return
			}
			slog.Info("indexer: done", "dir", d,
				"indexed", stats.Indexed,
				"skipped", stats.Skipped,
				"errors", stats.Errors)
		}()
		go watcher.New(d, database, searchCache).Run(ctx)
	}

	go func() {
		bgIndexer := embedder.NewBackgroundIndexer(embClient, database, database)
		bgIndexer.Run(ctx, 30*time.Second)
	}()

	// ── HTTP server ───────────────────────────────────────────────────────
	slog.Info("starting server", "addr", *addr, "version", Version)

	initMode := hwProfile.Mode.String()
	if snapshot.ModeOverride != "" {
		initMode = snapshot.ModeOverride
	}

	srv := server.New(*addr, initMode, Version, database, embClient, roots, *token, searchCache)
	srv.SetDBPath(*dbPath)
	srv.SetConfig(cfg)

	dirChangeCh := make(chan string, 4)
	srv.SetDirChangeCh(dirChangeCh)
	go func() {
		for {
			select {
			case newDir := <-dirChangeCh:
				go func(d string) {
					stats, err := indexer.Run(ctx, database, d)
					if err != nil {
						slog.Error("indexer: failed for new dir", "dir", d, "err", err)
						return
					}
					slog.Info("indexer: done for new dir", "dir", d, "indexed", stats.Indexed)
					go watcher.New(d, database, searchCache).Run(ctx)
				}(newDir)
			case <-ctx.Done():
				return
			}
		}
	}()

	// FIX 1: Wait for server readiness, then open browser (no race condition)
	if !*noOpen {
		go func() {
			if waitForServer(baseURL+"/health", 15*time.Second) {
				slog.Info("server ready, opening browser", "url", baseURL)
				openBrowser(baseURL)
			} else {
				slog.Warn("server did not become ready in time, skipping browser open")
			}
		}()
	}

	// Graceful shutdown: wait for OS signal then shutdown cleanly.
	go func() {
		<-ctx.Done()
		slog.Info("shutdown: signal received")
		sdCtx, sdCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer sdCancel()
		if err := srv.Shutdown(sdCtx); err != nil {
			slog.Error("shutdown: server error", "err", err)
		}
		if database != nil {
			database.Checkpoint()
			_ = database.Close()
		}
	}()

	if err := srv.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "err", err)
	}
}
