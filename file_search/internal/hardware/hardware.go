package hardware

import (
	"bufio"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Mode represents the detected capability level of the machine.
type Mode int

const (
	ModeEssential Mode = iota // Old/modest CPU, no GPU → FTS5 only
	ModeAdvanced              // Modern CPU with AVX2 + 8GB RAM → lightweight semantic
	ModePro                   // GPU (NVIDIA/AMD) + VRAM → full semantic
)

func (m Mode) String() string {
	switch m {
	case ModeAdvanced:
		return "Avancé"
	case ModePro:
		return "Pro"
	default:
		return "Essentiel"
	}
}

func (m Mode) Emoji() string {
	switch m {
	case ModeAdvanced:
		return "🟡"
	case ModePro:
		return "🟢"
	default:
		return "🔵"
	}
}

// Profile holds the raw detected hardware data.
type Profile struct {
	CPUCores  int
	RAMBytes  uint64 // total RAM in bytes
	HasAVX2   bool
	HasNVIDIA bool
	HasAMD    bool
	Mode      Mode
}

// Detect runs hardware detection and returns a Profile with the selected Mode.
func Detect() Profile {
	p := Profile{}
	p.CPUCores = runtime.NumCPU()
	p.RAMBytes = detectRAM()
	p.HasAVX2 = detectAVX2()
	p.HasNVIDIA = detectNVIDIA()
	p.HasAMD = detectAMD()
	p.Mode = classify(p)
	return p
}

// Log prints the hardware profile to slog at INFO level.
func (p Profile) Log() {
	ramGB := float64(p.RAMBytes) / (1024 * 1024 * 1024)
	slog.Info("hardware profile",
		"cpu_cores", p.CPUCores,
		"ram_gb", int(ramGB),
		"avx2", p.HasAVX2,
		"gpu_nvidia", p.HasNVIDIA,
		"gpu_amd", p.HasAMD,
		"mode", p.Mode.String(),
	)
	slog.Info("mode detected",
		"icon", p.Mode.Emoji(),
		"mode", p.Mode.String(),
		"semantic_available", p.Mode >= ModeAdvanced,
	)
}

// classify decides the mode based on hardware capabilities.
func classify(p Profile) Mode {
	const minRAMForAdvanced = 6 * 1024 * 1024 * 1024 // 6GB
	const minRAMForPro = 8 * 1024 * 1024 * 1024      // 8GB

	// Pro: GPU detected with enough RAM
	if (p.HasNVIDIA || p.HasAMD) && p.RAMBytes >= minRAMForPro {
		return ModePro
	}
	// Advanced: modern CPU with AVX2 and enough RAM and at least 4 cores
	if p.HasAVX2 && p.RAMBytes >= minRAMForAdvanced && p.CPUCores >= 4 {
		return ModeAdvanced
	}
	return ModeEssential
}

// detectRAM reads total RAM from /proc/meminfo on Linux.
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
			// format: "MemTotal:       16384000 kB"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				for _, c := range fields[1] {
					if c >= '0' && c <= '9' {
						kb = kb*10 + uint64(c-'0')
					}
				}
			}
			return kb * 1024
		}
	}
	return 0
}

// detectAVX2 checks /proc/cpuinfo for the "avx2" flag on Linux.
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
			return false // only check first "flags" line
		}
	}
	return false
}

// detectNVIDIA checks if nvidia-smi is available and returns exit 0.
func detectNVIDIA() bool {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// detectAMD checks for AMD GPU via /sys/class/drm or rocm-smi.
func detectAMD() bool {
	// Check via rocm-smi first
	cmd := exec.Command("rocm-smi", "--showid")
	if err := cmd.Run(); err == nil {
		return true
	}
	// Fallback: check /sys/class/drm for amdgpu
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return false
	}
	for _, e := range entries {
		driverLink := "/sys/class/drm/" + e.Name() + "/device/driver"
		target, err := os.Readlink(driverLink)
		if err == nil && strings.Contains(target, "amdgpu") {
			return true
		}
	}
	return false
}
