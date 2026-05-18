package indexer

import "syscall"

// Windows file attribute flags for cloud-tiered (online-only) files.
const (
	fileAttrRecallOnOpen       uint32 = 0x00040000 // opening triggers download
	fileAttrRecallOnDataAccess uint32 = 0x00400000 // reading triggers download
	fileAttrOffline             uint32 = 0x00001000 // legacy offline flag
)

// isCloudPlaceholder returns true when path is a OneDrive / cloud-sync online-only
// placeholder. Indexing such a file would trigger a network download or fail offline.
func isCloudPlaceholder(path string) bool {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(pathPtr)
	if err != nil {
		return false
	}
	return attrs&fileAttrRecallOnOpen != 0 ||
		attrs&fileAttrRecallOnDataAccess != 0 ||
		attrs&fileAttrOffline != 0
}
