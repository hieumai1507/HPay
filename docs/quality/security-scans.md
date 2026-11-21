# Static security scans

Run on 2026-11-21 against the working tree (uncommitted). Re-run before each release.

| Scan | Result |
|------|--------|
| `govulncheck v1.1.4` on `src/backend` (Go 1.25.14, in Docker) | 18 reachable findings. Every fix needs a newer toolchain or module: the standard library (`net/http`, `crypto/tls`, `crypto/x509`, `net/textproto`) is fixed in Go 1.26.9 to 1.26.11; `golang.org/x/net` v0.60.0 and `golang.org/x/text` v0.41.0 require Go 1.26; `excelize/v2` fixes exist only as pseudo-versions after v2.11.0. **Not applied**: it means a toolchain upgrade (Go 1.25 to 1.26), which is a decision for the owner (see the final report). |
| `npm audit --omit=dev` `src/admin` | Was 2 high (react-router CSRF in RSC mode). `npm audit fix` cleared it: 0 vulnerabilities. Admin vitest 20/20 and `tsc` still pass. |
| `npm audit --omit=dev` `src/mobile` | High findings in `@xmldom/xmldom` (transitive of the Expo toolchain). Not fixed: needs an Expo SDK or dependency bump. |
| `gosec`, `gitleaks` | Not installed on this machine; not run. |
| Manual secret check | `src/backend/.env` is git-ignored. Its `JWT_SECRET` is the public placeholder; Neon and Cloudinary credentials found earlier in a local `.env` must be rotated by the owner. |

`.claude/` hooks and `.gitnexus/` indexes are ignored by git.