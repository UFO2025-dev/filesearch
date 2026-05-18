package hardware

import (
	"log/slog"
	"runtime"
)

// Mode represents the detected capability level of the machine.
type Mode int

const (
	ModeEssential Mode = iota // Old/modest CPU, no GPU → FTS5 only
	ModeAdvanced              // Modern CPU with AVX2 + 6GB RAM → lightweight semantic
	ModePro                   // GPU (NVIDIA/AMD) + 8GB RAM → full semantic
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
	const minRAMForAdvanced = 6 * 1024 * 1024 * 1024 // 6 GB
	const minRAMForPro = 8 * 1024 * 1024 * 1024      // 8 GB

	if (p.HasNVIDIA || p.HasAMD) && p.RAMBytes >= minRAMForPro {
		return ModePro
	}
	if p.HasAVX2 && p.RAMBytes >= minRAMForAdvanced && p.CPUCores >= 4 {
		return ModeAdvanced
	}
	return ModeEssential
}
