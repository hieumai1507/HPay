# Final report: Waves 1 to 6 (working tree, NOT committed, NOT pushed)

Date: 2026-11-21. Nothing has been committed or pushed. Waiting for owner approval.

## Verified in this session
| Check | Result |
|-------|--------|
| Backend, full suite in Docker (`scripts\test-backend.ps1 -Db lead_check`, `-race -p 1`), last run after the offer-limits slice | all packages `ok`, integration package `ok`; migrations 000022 and 000023 verified up, down, up on a fresh database |
| `go build ./...`, `go vet ./...`, `gofmt -l` | clean (gofmt normalized 30 files, mostly a missing trailing newline) |
| Admin: vitest, `tsc --noEmit` | 20/20 pass, no type errors |
| Mobile: jest | 22 suites, 116/116 pass |
| `npm audit --omit=dev` admin | 0 vulnerabilities after `npm audit fix` |
| Mutation checks | each new guard (ledger, reversal, RBAC, SSRF, KYC image) has a test that fails when the guard is removed |

## Fixed (see docs/bugs/INDEX.md)
BE-C1 double posting guard (BUG-0001, ADR-0010); BE-H1 idempotency per user for transfer, wallet, money request (BUG-0002/0003, ADR-0011); BE-H7 explicit APP_ENV and environment guards (BUG-0010, ADR-0012); BE-H3/H4 reversal uniqueness and `needs_manual` (BUG-0011, ADR-0013); BE-C2 server-side PIN gate (BUG-0012, ADR-0014); BE-H6 admin RBAC from the database (BUG-0013, ADR-0015); SSRF guard for outbound webhooks (BUG-0014); KYC upload validation and file serving headers (BUG-0015, upload part of BE-H9). Admin and mobile apps: mocks only in dev, money helpers, idempotency keys, PIN seam, CSP.
Foreign-project names (TMDS, DEH) removed from code, seeds, tests, READMEs and guides; the demo organization is now "Acme Corp". They remain only in `docs/history`, `docs/audit`, `docs/planning`, mockups and the sentence that explains they are not part of this repo.
Added after the first version of this report: ADR-0010 to ADR-0015 accepted; `compensation_failed` workflow status (migration 000022) plus recovery of a stuck `compensating` instance (BR-WF-13, BR-WF-16); org topup and disbursement idempotency per user and bound to the request (BR-ORG-17, tests in `org_idempotency_test.go`, mutation-checked). One existing workflow test (`TestWorkflowHappyPath`) counted calls across a shared database and failed whenever earlier runs had left events behind; it now counts per instance.
KYC limits attached to offers (ADR-0007, `docs/specs/w3-offer-limits`): migration 000023 adds `OFFER_CUST_L0` and the approved L0/L1/L2 limits; KYC approval or rejection moves the offer in the same transaction; a user without an offer row uses L0; an inactive offer blocks (`ERR_OFFER_INACTIVE`); `wallet.TxnLimit` is removed and every money path uses the offer; `/users/me` returns the offer limits. Demo seed users were remapped. Eight new tests, two mutations checked (default offer, approval overwriting any offer). Two existing tests assumed the old default (L2, 100M) and now assign an offer explicitly.
Fraud decision: verified already fail-closed (rule errors are retryable; the LLM advisory is asynchronous and never affects the verdict). No change needed.

## NOT done or NOT verified
- **Go toolchain**: `govulncheck` reports 18 reachable findings, all fixed only in Go 1.26.x (stdlib, `x/net`, `x/text`) or in excelize pseudo-versions. Needs your decision to move from Go 1.25 to 1.26. Details: `docs/quality/security-scans.md`.
- Mobile `npm audit`: high findings in `@xmldom/xmldom` (Expo toolchain), not fixed.
- `gosec` and `gitleaks` are not installed; not run. A regex secret scan over the 808 tracked and untracked files found nothing, and no `.env` is tracked. GitNexus `detect_changes` ran: 471 changed symbols in 160 files, risk "critical" only because the whole change set is uncommitted; affected flows are the ones changed on purpose (ledger `Execute`/`Topup`, `NewRouter`, mobile API client and screens).
- Playwright e2e was not run (needs backend, seed and browsers); admin e2e `orgs.spec.ts` was only renamed.
- QA workbooks: only TC-IDM cases were added; TC-LED/ENV/REV/AUT/RBA/SSRF/KYC case sets are in the spec folders but not merged into the xlsx files and no results recorded.


- 2FA provider: not available; the 2fa band is refused.
- PIN reset flow and mobile set-PIN screen not built.
- BE-H8 login lockout and timing; BE-H9 remainder (private Cloudinary delivery, separate file-signing key, identity-bound audited file access, EXIF, orphan cleanup).
- Weekly rolling limits from the approved T0/T1/T2 table (the offer model has daily and monthly only), balance-ceiling enforcement, currency model (a fixed 25,000 VND per USD was assumed), per-tier auth thresholds, automatic assignment of the L2 offer, and compliance review per jurisdiction.
- Admin UI for `needs_manual` approvals.
- Pre-existing lint errors (admin 6, mobile 3).
- `docs/reference/api.md` still has a section on unreachable endpoints that needs a note about what was fixed in this tree.

## Decisions for you
1. ADR-0010 to ADR-0015 are approved by you (2026-11-21) and marked `accepted`.
2. Breaking changes: `APP_ENV` is required; confirming a payment now needs a PIN, so users must set one; the 2fa band is refused.
3. Go 1.26 upgrade for the vulnerability findings.
4. Key rotation (Neon, Cloudinary, JWT) is postponed by you. The local `src/backend/.env` is git-ignored and must stay out of the commit.
5. Whether the remaining items above are in scope before the first push or follow-up work.

## Cleanup before commit
Remove outside the repo: `D:\hpay-git-backup`, `D:\hpay-wt-backup`, `D:\hpay-skills-research`, `D:\hpay-skills-archive`; scratch `D:\hpay\.gotmp` is ignored. Run `detect_changes` and a secret scan, then commit and push to `https://github.com/hieumai1507/HPay.git` only after your go-ahead.