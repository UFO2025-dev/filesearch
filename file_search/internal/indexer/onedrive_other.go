//go:build !windows

package indexer

// isCloudPlaceholder always returns false on non-Windows platforms.
// OneDrive cloud-tiered files only exist on Windows.
func isCloudPlaceholder(_ string) bool { return false }
