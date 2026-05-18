package indexer

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"gatewatch/file_search/internal/db"

	"github.com/ledongthuc/pdf"
)

const (
	maxFileSize   = 50 * 1024 * 1024 // 50MB
	maxIndexBytes = 1 * 1024 * 1024  // 1MB of text indexed per file — avoids loading giant CSVs/logs into RAM
	workersMaxCap = 8
)

var defaultExcludedDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".hg":          true,
	".svn":         true,
	"__pycache__":  true,
	".cache":       true,
	"dist":         true,
	"build":        true,
	"vendor":       true,
	".venv":        true,
	"venv":         true,
	".tox":         true,
	"target":       true,
	"bin":          true,
	"obj":          true,
	"site-packages": true,
	// Windows system directories — never index these
	"Windows":                  true,
	"System32":                 true,
	"SysWOW64":                 true,
	"WinSxS":                   true,
	"$Recycle.Bin":              true,
	"System Volume Information": true,
	"Recovery":                 true,
	"ProgramData":               true,
	"Program Files":             true,
	"Program Files (x86)":       true,
}

// secretExcludedDirs are directory NAMES (basename) that contain credentials or
// sensitive data. ALWAYS skipped — non-configurable hard policy.
var secretExcludedDirs = map[string]bool{
	".ssh":             true,
	".gnupg":           true,
	".gpg":             true,
	".aws":             true,
	".azure":           true,
	".kube":            true,
	".bitcoin":         true,
	".ethereum":        true,
	".electrum":        true,
	"User Data":        true, // Chrome/Edge password store
	"keystore":         true,
	".password-store":  true,
	".gnome-keyring":   true,
}

// secretExcludedPathSuffixes are lowercased forward-slash path suffixes matched
// against the full path to catch deeply nested credential directories.
var secretExcludedPathSuffixes = []string{
	"/.ssh",
	"/.gnupg",
	"/.aws",
	"/.azure",
	"/.kube",
	"/.bitcoin",
	"/.ethereum",
	"/google/chrome/user data/default",
	"/mozilla/firefox",
	"/microsoft/edge/user data/default",
	"/appdata/roaming/microsoft/credentials",
	"/appdata/roaming/microsoft/protect",
	"/appdata/local/microsoft/credentials",
}

// secretFileBasenames are exact filenames (lowercased) that must never be indexed.
var secretFileBasenames = map[string]bool{
	".env":         true,
	".env.local":   true,
	".env.prod":    true,
	".env.staging": true,
	".envrc":       true,
	"credentials":  true,
	".netrc":       true,
	".pgpass":      true,
	"id_rsa":       true,
	"id_ed25519":   true,
	"id_ecdsa":     true,
	"id_dsa":       true,
}

// secretFileExtensions are file extensions for private keys / keystores.
// Always skipped regardless of supportedExtensions.
var secretFileExtensions = map[string]bool{
	".pem":  true,
	".key":  true,
	".p12":  true,
	".pfx":  true,
	".p8":   true,
	".jks":  true,
	".kdbx": true,
	".asc":  true,
}

func adaptiveWorkers() int {
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	if n > workersMaxCap {
		n = workersMaxCap
	}
	return n
}

var (
	pdfOnce      sync.Once
	pdfAvailable bool
)

var supportedExtensions = map[string]bool{
	// Plain text
	".txt":  true,
	".md":   true,
	".html": true,
	".htm":  true,
	".csv":  true,
	".json": true,
	".yaml": true,
	".yml":  true,
	".rtf":  true,
	// PDF
	".pdf": true,
	// Office Open XML (Word, Excel, PowerPoint)
	".docx": true,
	".xlsx": true,
	".pptx": true,
	// OpenDocument (LibreOffice)
	".odt": true,
	".ods": true,
	".odp": true,
	// Plain text (code and config)
	".xml":  true,
	".log":  true,
	".ini":  true,
	".cfg":  true,
	".conf": true,
	".toml": true,
	".sh":   true,
	".bat":  true,
	".ps1":  true,
	".py":   true,
	".js":   true,
	".ts":   true,
	".sql":  true,
	".tex":  true,
	// Legacy Office formats (binary string extraction)
	".doc": true,
	".xls": true,
	".ppt": true,
}

type Stats struct {
	Indexed           int
	Skipped           int
	SkippedLarge      int
	Errors            int
	Duration          time.Duration
	SkippedExtensions map[string]int
}

func Run(ctx context.Context, database *db.DB, root string, extraExclude ...string) (Stats, error) {
	if _, err := os.Stat(root); err != nil {
		return Stats{}, fmt.Errorf("indexer: root %q not found: %w", root, err)
	}

	start := time.Now()
	var stats Stats
	stats.SkippedExtensions = make(map[string]int)

	excluded := make(map[string]bool, len(defaultExcludedDirs)+len(extraExclude))
	for k, v := range defaultExcludedDirs {
		excluded[k] = v
	}
	for _, e := range extraExclude {
		if e != "" {
			excluded[strings.TrimSpace(e)] = true
		}
	}

	workers := adaptiveWorkers()
	slog.Debug("indexer: worker pool", "workers", workers)
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			slog.Warn("walk error", "path", path, "err", err)
			mu.Lock(); stats.Errors++; mu.Unlock()
			return nil
		}
		if d.IsDir() {
			if excluded[filepath.Base(path)] {
				slog.Debug("indexer: skipping excluded dir", "dir", path)
				return fs.SkipDir
			}
			// Hard-exclude secret directories (SSH keys, credentials, crypto wallets…).
			baseName := filepath.Base(path)
			if secretExcludedDirs[baseName] {
				slog.Debug("indexer: skipping secret dir", "dir", path)
				return fs.SkipDir
			}
			// Also check full lowercased path for deeply nested credential dirs.
			lowerPath := strings.ToLower(filepath.ToSlash(path))
			for _, suffix := range secretExcludedPathSuffixes {
				if strings.HasSuffix(lowerPath, suffix) || strings.Contains(lowerPath, suffix+"/") {
					slog.Debug("indexer: skipping secret path", "dir", path)
					return fs.SkipDir
				}
			}
			// Skip directories named "env" that look like Python venvs
			if baseName == "env" {
				if _, err := os.Stat(filepath.Join(path, "pyvenv.cfg")); err == nil {
					slog.Debug("indexer: skipping Python venv", "dir", path)
					return fs.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		// Hard-exclude secret file extensions (private keys, keystores, etc.).
		if secretFileExtensions[ext] {
			slog.Debug("indexer: skipping secret file ext", "path", path, "ext", ext)
			mu.Lock(); stats.Skipped++; mu.Unlock()
			return nil
		}
		// Hard-exclude known secret filenames (.env, id_rsa, credentials, etc.).
		if secretFileBasenames[strings.ToLower(filepath.Base(path))] {
			slog.Debug("indexer: skipping secret filename", "path", path)
			mu.Lock(); stats.Skipped++; mu.Unlock()
			return nil
		}
		if !supportedExtensions[ext] {
			mu.Lock()
			stats.Skipped++
			if ext != "" {
				stats.SkippedExtensions[ext]++
			}
			mu.Unlock()
			return nil
		}
		info, err := d.Info()
		if err != nil {
			slog.Warn("stat error", "path", path, "err", err)
			mu.Lock(); stats.Errors++; mu.Unlock()
			return nil
		}
		if info.Size() > maxFileSize {
			slog.Warn("skipping large file", "path", path, "size_mb", info.Size()/1024/1024)
			mu.Lock()
			stats.Skipped++
			stats.SkippedLarge++
			mu.Unlock()
			return nil
		}
		// Skip OneDrive / cloud-storage online-only placeholders (Windows).
		// Accessing them would trigger a network download or fail when offline.
		if isCloudPlaceholder(path) {
			slog.Debug("indexer: skipping cloud placeholder", "path", path)
			mu.Lock(); stats.Skipped++; mu.Unlock()
			return nil
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := IndexFile(ctx, database, p); err != nil {
				slog.Error("index error", "err", err)
				mu.Lock(); stats.Errors++; mu.Unlock()
				return
			}
			slog.Debug("indexed", "path", p)
			mu.Lock(); stats.Indexed++; mu.Unlock()
		}(path)
		return nil
	})
	wg.Wait()

	if len(stats.SkippedExtensions) > 0 {
		slog.Info("indexer: skipped extensions summary", "extensions", stats.SkippedExtensions)
	}
	if stats.SkippedLarge > 0 {
		slog.Warn("indexer: files skipped due to size limit", "count", stats.SkippedLarge, "limit_mb", maxFileSize/1024/1024)
	}

	stats.Duration = time.Since(start)
	return stats, err
}

func IndexFile(ctx context.Context, database *db.DB, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Size() > maxFileSize {
		return fmt.Errorf("file too large (%dMB): %s", info.Size()/1024/1024, path)
	}
	content, err := extractText(ctx, path)
	if err != nil {
		return fmt.Errorf("extract %s: %w", path, err)
	}
	// Prepend filename so searching by filename always works
	filename := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	content = filename + "\n" + nameWithoutExt + "\n" + content
	if strings.TrimSpace(content) == "" {
		return nil
	}
	return database.UpsertWithMtime(ctx, path, content, info.ModTime().Unix())
}

func extractText(ctx context.Context, path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return extractPDF(ctx, path)
	case ".md":
		return extractMarkdown(path)
	case ".html", ".htm":
		return extractHTML(path)
	case ".docx":
		return extractDOCX(path)
	case ".xlsx":
		return extractXLSX(path)
	case ".pptx":
		return extractPPTX(path)
	case ".odt", ".ods", ".odp":
		return extractODF(path)
	case ".rtf":
		return extractRTF(path)
	case ".doc", ".xls", ".ppt":
		return extractLegacyOffice(path)
	case ".xml":
		return extractHTML(path) // XML is HTML-like, strip tags
	default:
		// .txt, .csv, .json, .yaml, .yml — plain text
		return extractRaw(path)
	}
}

// ── Plain text ─────────────────────────────────────────────────────────────

func extractRaw(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 64*1024) // 64 KB per line — handles wide CSV rows
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
		if sb.Len() >= maxIndexBytes {
			break // cap at 1 MB — large CSVs/logs are keyword-searchable without loading all into RAM
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan: %w", err)
	}
	return sb.String(), nil
}

// ── PDF ────────────────────────────────────────────────────────────────────

func extractPDF(ctx context.Context, path string) (string, error) {
	// Try pdftotext first (higher quality, available on Linux with poppler-utils).
	pdfOnce.Do(func() {
		_, err := exec.LookPath("pdftotext")
		pdfAvailable = err == nil
	})
	if pdfAvailable {
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", path, "-")
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			return out.String(), nil
		}
	}
	// Pure Go fallback — works on Windows and any OS without poppler.
	return extractPDFGo(path)
}

// extractPDFGo extracts plain text from a PDF using pure Go (no external tools).
func extractPDFGo(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("pdf open: %w", err)
	}
	defer f.Close()
	var buf strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		if p := r.Page(i); !p.V.IsNull() {
			if text, err := p.GetPlainText(nil); err == nil {
				buf.WriteString(text)
			}
		}
	}
	return buf.String(), nil
}

// ── Markdown ───────────────────────────────────────────────────────────────

var (
	mdHeaderRe    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	mdEmphRe      = regexp.MustCompile(`[*_]{1,3}([^*_]+)[*_]{1,3}`)
	mdLinkRe      = regexp.MustCompile(`!?\[([^\]]+)\]\([^)]+\)`)
	mdCodeRe      = regexp.MustCompile("`{1,3}[^`]*`{1,3}")
	mdHtmlTagRe   = regexp.MustCompile(`<[^>]+>`)
)

func extractMarkdown(path string) (string, error) {
	raw, err := extractRaw(path)
	if err != nil {
		return "", err
	}
	s := mdHeaderRe.ReplaceAllString(raw, "")
	s = mdEmphRe.ReplaceAllString(s, "$1")
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = mdCodeRe.ReplaceAllString(s, " ")
	s = mdHtmlTagRe.ReplaceAllString(s, " ")
	return s, nil
}

// ── HTML ───────────────────────────────────────────────────────────────────

var (
	htmlTagRe     = regexp.MustCompile(`(?i)<[^>]+>`)
	htmlScriptRe  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	htmlStyleRe   = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlCommentRe = regexp.MustCompile(`<!--.*?-->`)
)

func extractHTML(path string) (string, error) {
	raw, err := extractRaw(path)
	if err != nil {
		return "", err
	}
	s := htmlScriptRe.ReplaceAllString(raw, " ")
	s = htmlStyleRe.ReplaceAllString(s, " ")
	s = htmlCommentRe.ReplaceAllString(s, " ")
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s), nil
}

// ── DOCX (Word) ────────────────────────────────────────────────────────────
// A .docx is a ZIP archive. The text lives in word/document.xml.

func extractDOCX(path string) (string, error) {
	return extractZipXML(path, func(name string) bool {
		return name == "word/document.xml"
	})
}

// ── XLSX (Excel) ───────────────────────────────────────────────────────────
// Shared strings are in xl/sharedStrings.xml; inline strings in xl/worksheets/*.xml.

func extractXLSX(path string) (string, error) {
	return extractZipXML(path, func(name string) bool {
		return name == "xl/sharedStrings.xml" ||
			strings.HasPrefix(name, "xl/worksheets/")
	})
}

// ── PPTX (PowerPoint) ──────────────────────────────────────────────────────
// Text is in ppt/slides/slide*.xml and ppt/notes/notesSlide*.xml.

func extractPPTX(path string) (string, error) {
	return extractZipXML(path, func(name string) bool {
		return strings.HasPrefix(name, "ppt/slides/slide") ||
			strings.HasPrefix(name, "ppt/notes/")
	})
}

// ── ODF (LibreOffice odt/ods/odp) ─────────────────────────────────────────
// All text content is in content.xml.

func extractODF(path string) (string, error) {
	return extractZipXML(path, func(name string) bool {
		return name == "content.xml"
	})
}

// extractZipXML opens a ZIP archive, reads all matching XML files,
// and returns the plain text content by stripping all XML tags.
func extractZipXML(path string, match func(string) bool) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open zip %s: %w", path, err)
	}
	defer r.Close()

	var sb strings.Builder
	for _, f := range r.File {
		if !match(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		text, err := xmlToText(rc)
		rc.Close()
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// xmlToText uses the standard xml.Decoder to extract all CharData (text nodes),
// ignoring all tags. This is the most robust way to extract text from Office XML.
func xmlToText(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Non-fatal: some Office files have minor XML quirks
			break
		}
		if cd, ok := tok.(xml.CharData); ok {
			text := strings.TrimSpace(string(cd))
			if text != "" {
				sb.WriteString(text)
				sb.WriteByte(' ')
			}
		}
	}
	return sb.String(), nil
}

// ── RTF ────────────────────────────────────────────────────────────────────
// Strip RTF control words (\word), groups ({...}), and binary data.

var (
	rtfControlRe = regexp.MustCompile(`\\[a-z]+[-]?\d*\s?`)
	rtfGroupRe   = regexp.MustCompile(`[{}]`)
	rtfBinRe     = regexp.MustCompile(`\\bin\d+`)
)

func extractRTF(path string) (string, error) {
	raw, err := extractRaw(path)
	if err != nil {
		return "", err
	}
	s := rtfBinRe.ReplaceAllString(raw, " ")
	s = rtfControlRe.ReplaceAllString(s, " ")
	s = rtfGroupRe.ReplaceAllString(s, " ")
	// Remove remaining backslash escapes like \'e9
	s = regexp.MustCompile(`\\'[0-9a-fA-F]{2}`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s), nil
}

// extractLegacyOffice extracts readable text strings from binary OLE Office files (.doc, .xls, .ppt).
// It uses a printable-string scan (similar to Unix `strings`) — not perfect but useful for keyword search.
func extractLegacyOffice(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	run := make([]byte, 0, 64)
	for i, b := range data {
		if (b >= 32 && b < 127) || b == '\n' || b == '\r' || b == '\t' {
			run = append(run, b)
		} else {
			if len(run) >= 6 {
				sb.Write(run)
				sb.WriteByte('\n')
			}
			run = run[:0]
		}
		_ = i
	}
	if len(run) >= 6 {
		sb.Write(run)
	}
	return sb.String(), nil
}
