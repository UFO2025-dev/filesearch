//go:build windows

package hardware

import (
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MEMORYSTATUSEX is the Windows struct filled by GlobalMemoryStatusEx.
// https://learn.microsoft.com/windows/win32/api/sysinfoapi/ns-sysinfoapi-memorystatusex
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

func detectRAM() uint64 {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	globalMemoryStatusEx := kernel32.NewProc("GlobalMemoryStatusEx")

	var ms memoryStatusEx
	ms.dwLength = uint32(unsafe.Sizeof(ms))
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0
	}
	return ms.ullTotalPhys
}

// detectAVX2 uses CPUID instruction via golang.org/x/sys/cpu.
// We call it indirectly to avoid importing x/sys/cpu directly
// (it pulls in assembly that requires CGO on some platforms).
// Instead we use the IsProcessorFeaturePresent Win32 API.
// PF_AVX2_INSTRUCTIONS_AVAILABLE = 40
func detectAVX2() bool {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	isProcessorFeaturePresent := kernel32.NewProc("IsProcessorFeaturePresent")
	const pfAVX2 = 40
	ret, _, _ := isProcessorFeaturePresent.Call(uintptr(pfAVX2))
	return ret != 0
}

func detectNVIDIA() bool {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func detectAMD() bool {
	// On Windows, check via wmic for AMD GPU in display adapters.
	cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name", "/format:list")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(out))
	return strings.Contains(lower, "amd") || strings.Contains(lower, "radeon")
}
