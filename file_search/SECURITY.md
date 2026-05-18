# Security Policy

## Supported Versions

| Version | Supported |
|---|---|
| 1.2.x (current) | ✅ Active |
| 1.1.x | ⚠️ Critical fixes only |
| < 1.1 | ❌ |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Use GitHub's [private security advisory](https://github.com/UFO2025-dev/gatewatch_mvp/security/advisories/new) to report privately.

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (optional)

We aim to respond within 72 hours and publish a fix within 14 days for critical issues.

---

## Security Model

FileSearch is a **local-only, single-user desktop application**. Its threat model is:

### In scope
- Local privilege escalation via path traversal
- Indexing of credentials/secrets
- Unauthenticated API access by other local processes
- Log injection
- ZIP/path traversal in diagnostics bundle

### Out of scope (by design)
- Network attacks (no external exposure by default)
- Multi-user isolation (single-user app)
- DB encryption at rest (roadmap v1.3)

---

## Implemented Security Controls

All controls verified against source code.

### Path Validation (`internal/server/middleware.go`)
- UNC path rejection (`\\server\share`)
- OS path rejection: `Windows/System32`, `/etc`, `/sys`, `/proc`, `/root`, `/var/log`
- Library path rejection: `AppData`, `Program Files`, `/usr`, `/lib`, `/opt`

### Secrets Exclusion (`internal/indexer/indexer.go`)
Hard-excluded directories and files — **non-configurable**:
- Dirs: `.ssh/`, `.gnupg/`, `.aws/`, `.azure/`, `.kube/`, `credentials/`, `secrets/`
- Files: `id_rsa`, `id_ed25519`, `.env`, `.env.local`, `.env.prod`, `.envrc`, `.netrc`, `credentials`
- Extensions: `.pem`, `.key`, `.pfx`, `.p12`, `.jks`, `.keystore`, `.pkcs12`, `.cer`

### API Security (`internal/server/middleware.go`)
- **Rate limiting**: Token bucket, 100 req/s per IP, automatic 429 response
- **CSRF**: X-CSRF-Token header required on all POST/PUT/DELETE requests
- **Bearer auth**: Enabled when `-token=` flag is set at launch
- **Panic recovery**: All panics caught, logged, return 500 (no stack trace leaked to client)
- **Health endpoint**: Public by design (needed for startup checks), no sensitive data exposed

### OneDrive / Cloud Files (`internal/indexer/onedrive_windows.go`)
Windows `GetFileAttributes` syscall checks:
- `FILE_ATTRIBUTE_RECALL_ON_OPEN` (0x40000)
- `FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS` (0x400000)
- `FILE_ATTRIBUTE_OFFLINE` (0x1000)

Cloud-only files are silently skipped — no network download triggered during indexing.

---

## Known Limitations

| Limitation | Status |
|---|---|
| DB stored unencrypted | Roadmap v1.3 — AES-256 at rest |
| Auth disabled by default | By design for desktop; `-token=` flag available |
| No TLS | Local-only; TLS roadmap for remote usage |
| No multi-user isolation | Single-user app; enterprise multi-user roadmap v2.0 |
| `govulncheck` runs on every CI push | See `.github/workflows/ci.yml` |
