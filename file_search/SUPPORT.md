# Support & Bug Reports

## Before Reporting

1. **Check the version**: version badge shown in the top-right of the UI, or `GET /api/version`
2. **Download diagnostics bundle**: Settings tab → "📦 Download diagnostic bundle"  
   Contains: `info.json`, `config.json`, `audit.csv`, last 500 log lines

## Bug Report

Open a [GitHub Issue](https://github.com/UFO2025-dev/gatewatch_mvp/issues/new) with:

```
FileSearch version: (from UI or /api/version)
OS: Windows 10/11 / Linux distro
Go version (if built from source):

Steps to reproduce:
1.
2.
3.

Expected behavior:

Actual behavior:

Diagnostic bundle: (attach the ZIP from Settings → Diagnostics)
```

## Common Issues

### App doesn't start / blank browser
- Check if port 7890 is in use: `netstat -ano | findstr 7890`
- Check log file: `%APPDATA%\FileSearchilesearch.log`

### Files not appearing in search
- Verify the folder is added in Settings
- Check if the file extension is supported (see README)
- OneDrive online-only files are intentionally skipped

### Search returns no results after adding a folder
- Indexing may still be in progress — watch the progress bar
- Check if the folder is accessible (no permission errors in logs)

### "Too Many Requests" error
- The rate limiter allows 100 requests/second per IP
- If you're scripting against the API, add delays between requests

## Diagnostics Bundle Contents

| File | Contents |
|---|---|
| `info.json` | Version, Go runtime, OS, hardware mode, file count, DB size |
| `config.json` | Current configuration (indexed roots, mode override) |
| `audit.csv` | Last 100 searches with timestamps and result counts |
| `filesearch.log` | Last 500 log lines (Windows only) |

## Security Issues

Do **not** open a public issue. Use [GitHub Security Advisories](https://github.com/UFO2025-dev/gatewatch_mvp/security/advisories/new).
