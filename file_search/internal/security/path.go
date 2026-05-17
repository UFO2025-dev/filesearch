package security

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrOutsideRoot is returned when a path is outside the allowed root directory.
var ErrOutsideRoot = errors.New("path is outside the allowed root directory")

// ValidatePath ensures that target resolves to a path inside root.
// It guards against:
//   - path traversal attacks (e.g. ../../etc/passwd)
//   - symlink escape attacks (e.g. ln -s /etc/passwd docs/evil.txt)
func ValidatePath(root, target string) error {
	cleanRoot := filepath.Clean(root)

	// Resolve symlinks in target before any string comparison.
	// Without this, a symlink pointing outside root would bypass the check.
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		// File may not exist yet (e.g. being created); fall back to Clean only.
		resolvedTarget = filepath.Clean(target)
	}

	// filepath.Rel returns a relative path from root to target.
	// If the result starts with ".." the target escapes root.
	rel, err := filepath.Rel(cleanRoot, resolvedTarget)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOutsideRoot, err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %q", ErrOutsideRoot, target)
	}
	return nil
}
