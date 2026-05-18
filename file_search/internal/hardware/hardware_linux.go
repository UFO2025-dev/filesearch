//go:build linux

package hardware

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

func detectRAM() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			var kb uint64
			for _, c := range strings.Fields(line)[1] {
				if c >= '0' && c <= '9' {
					kb = kb*10 + uint64(c-'0')
				}
			}
			return kb * 1024
		}
	}
	return 0
}

func detectAVX2() bool {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "flags") {
			for _, flag := range strings.Fields(line) {
				if flag == "avx2" {
					return true
				}
			}
			return false
		}
	}
	return false
}

func detectNVIDIA() bool {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func detectAMD() bool {
	if err := exec.Command("rocm-smi", "--showid").Run(); err == nil {
		return true
	}
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return false
	}
	for _, e := range entries {
		target, err := os.Readlink("/sys/class/drm/" + e.Name() + "/device/driver")
		if err == nil && strings.Contains(target, "amdgpu") {
			return true
		}
	}
	return false
}
