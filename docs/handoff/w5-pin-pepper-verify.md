---
title: PIN pepper verification
type: handoff
status: done
last_verified: 2026-11-25
spec: ../specs/w5-pin-pepper/spec.md
---

# w5-pin-pepper verification (2026-11-25)

Counts: 29 Pass, 0 Fail, 1 Blocked (TC-PEP-029, the runbook count query needs a person with psql), 0 Not Run.

Commands run in this session (Docker runner, database pep1 then pep2):
- `.\scripts\test-backend.ps1 -Db pep1 -GoArgs "./internal/auth ./internal/platform/config -race -count=1"` ok.
- `.\scripts\test-backend.ps1 -Db pep1 -GoArgs "./test/integration/authn -race -count=1"` ok.
- `.\scripts\test-backend.ps1 -Db pep2 -GoArgs "./... -race -count=1"` every package ok.
- gofmt on all touched Go files (copied with LF endings) printed nothing; no file starts with a BOM.
- Mutation checks, each reverted afterwards: parser ignores `k=` fails 5 tests; unconditional rewrite (guard removed from `RehashUserPIN`) fails `TestPinRehash_ConcurrentChangeNotOverwritten`; rewrite after a failed verification fails `TestPinRehash_WrongPinDoesNotRewrite`.
- Load test before and after: see the performance note.

Not run: `go vet` and `go build` outside the Docker test run (the compile inside `go test` passed), `python scripts/check-openapi-sync.py` (no endpoint changed), the count query, a real rotation drill, a deployment with a real secret manager.