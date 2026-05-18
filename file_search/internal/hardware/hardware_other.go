//go:build !linux && !windows

package hardware

import (
	"os/exec"
	"strconv"
	"strings"
)

// detectRAM uses sysctl on macOS/BSD.
func detectRAM() uint64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func detectAVX2() bool {
	// macOS: sysctl machdep.cpu.features
	out, err := exec.Command("sysctl", "-n", "machdep.cpu.leaf7_features").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "avx2")
}

func detectNVIDIA() bool {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func detectAMD() bool {
	return false // not reliably detectable cross-platform without CGO
}
