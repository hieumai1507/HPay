# Verification: PIN reset (ADR-0016)

Date 2026-11-21. Spec `docs/specs/w4-pin-reset/spec.md`, cases `test-cases.md`, workbook `04-platform-crosscutting.xlsx` rows TC-PRS-001 to 020.

## Results
| Check | Result |
|---|---|
| Cases TC-PRS-001 to 020 | 20 Pass, 0 Fail, 0 Blocked, 0 Not Run (recorded in the workbook) |
| Full backend suite with `-race -p 1` and a 4-connection pool (CI-like) | all packages ok, including `test/integration/{admin,authn,kyc,money,org}` |
| `go build ./...`, `go vet ./...`, `gofmt -l` | clean (run in Docker, Go 1.26) |
| `python scripts/check-openapi-sync.py`, OpenAPI YAML parse | clean |
| Admin: `tsc --noEmit`, eslint, vitest | 0 type errors, 0 lint errors (6 old warnings), 20 of 20 tests |
| RBAC drift test (Go table equals admin-app table) | pass with the new `pin.reset` |

## Mutation checks (each made a test fail, then the code was restored)
- Remove the password step-up: `TestPINReset_WrongPasswordSendsNothing` fails.
- Remove the guarded consume from the SQL: `TestPINReset_GuardedConsumeHasOneWinner` fails (16 winners instead of 1). The service-level concurrency test alone did NOT catch this, because eight parallel confirms also exhaust the three-attempt counter, so the store-level test was added.
- Do not call the cooling check in the request path: `TestPINReset_CoolingCapAppliesToConfirm` fails.
- Give `ops` the `pin.reset` permission: `TestAdminClearPIN_OnlySuperadmin` fails.

## Not verified
- No real SMS or e-mail delivery exists. Production and demo use the refusing sender, so reset is unavailable there until a provider is chosen (separate ADR with 2FA).
- No mobile or admin screen was built; only the API.
- CI result for the commit is checked after the push.