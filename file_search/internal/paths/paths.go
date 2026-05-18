package paths

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const AppName = "FileSearch"

// AppDirs holds platform-appropriate directories for FileSearch data.
//
//   Windows:
//     ConfigDir  = %APPDATA%\FileSearch          (roaming — synced across machines)
//     DataDir    = %LOCALAPPDATA%\FileSearch      (local   — large files, not synced)
//     LogDir     = %LOCALAPPDATA%\FileSearch\logs
//
//   Linux / macOS:
//     ConfigDir  = $XDG_CONFIG_HOME/FileSearch    (~/.config/FileSearch)
//     DataDir    = $XDG_DATA_HOME/FileSearch      (~/.local/share/FileSearch)
//     LogDir     = DataDir/logs
type AppDirs struct {
	ConfigDir string
	DataDir   string
	LogDir    string
}

// Resolve returns platform-appropriate directories and creates them (0700) if needed.
func Resolve() (AppDirs, error) {
	cfgRoot, err := os.UserConfigDir()
	if err != nil {
		return AppDirs{}, fmt.Errorf("paths: cannot determine config dir: %w", err)
	}

	var dataRoot string
	if runtime.GOOS == "windows" {
		if ld := os.Getenv("LOCALAPPDATA"); ld != "" {
			dataRoot = ld
		} else {
			// Fallback: same bucket as config (rare — LOCALAPPDATA should always be set)
			dataRoot = cfgRoot
		}
	} else {
		// XDG_DATA_HOME, or ~/.local/share
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			dataRoot = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return AppDirs{}, fmt.Errorf("paths: cannot determine home dir: %w", err)
			}
			dataRoot = filepath.Join(home, ".local", "share")
		}
	}

	d := AppDirs{
		ConfigDir: filepath.Join(cfgRoot, AppName),
		DataDir:   filepath.Join(dataRoot, AppName),
		LogDir:    filepath.Join(dataRoot, AppName, "logs"),
	}

	for _, dir := range []string{d.ConfigDir, d.DataDir, d.LogDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return AppDirs{}, fmt.Errorf("paths: cannot create dir %s: %w", dir, err)
		}
	}

	return d, nil
}

// ConfigFile returns the full path to the persistent config JSON.
func (d AppDirs) ConfigFile() string { return filepath.Join(d.ConfigDir, "config.json") }

// DBFile returns the full path to the SQLite index database.
func (d AppDirs) DBFile() string { return filepath.Join(d.DataDir, "index.db") }

// MigrateIfNeeded copies files from the legacy exe-relative layout
// (<exeDir>/data/) to the new platform paths, if the legacy files exist
// and the new destinations are empty. Safe to call on every startup — it
// is a no-op once migration has already been done.
//
// Returns true if a migration was performed (caller should log/notify user).
func MigrateIfNeeded(exeDir string, d AppDirs) (bool, error) {
	migrated := false

	pairs := []struct{ oldPath, newPath string }{
		{filepath.Join(exeDir, "data", "index.db"), d.DBFile()},
		{filepath.Join(exeDir, "data", "config.json"), d.ConfigFile()},
	}

	for _, p := range pairs {
		if _, err := os.Stat(p.oldPath); os.IsNotExist(err) {
			continue // nothing to migrate
		}
		if _, err := os.Stat(p.newPath); err == nil {
			continue // destination already exists — skip
		}
		if err := copyFile(p.oldPath, p.newPath); err != nil {
			return migrated, fmt.Errorf("paths: migration %s -> %s: %w", p.oldPath, p.newPath, err)
		}
		migrated = true
	}

	return migrated, nil
}

// copyFile copies src to dst atomically (write to tmp then rename).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
