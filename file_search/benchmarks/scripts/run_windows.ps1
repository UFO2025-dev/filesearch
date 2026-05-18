# run_windows.ps1 — Run all benchmarks on Windows.
$ErrorActionPreference = "Stop"
$ResultsDir = "$PSScriptRoot\..\results"
$Timestamp  = Get-Date -Format "yyyyMMdd_HHmmss"
$Arch       = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "x86" }
$OutFile    = "$ResultsDir\${Timestamp}_${Arch}.txt"

New-Item -ItemType Directory -Force -Path $ResultsDir | Out-Null

Write-Host "=== FileSearch Benchmark Suite (Windows) ==="
Write-Host "Go: $(go version)"
Write-Host "Time: $(Get-Date -Format u)"

go test `
  -run="^$" `
  -bench="." `
  -benchmem `
  -benchtime=5s `
  -count=3 `
  ./benchmarks/ |
  Tee-Object -FilePath $OutFile

Write-Host ""
Write-Host "Results saved to: $OutFile"
