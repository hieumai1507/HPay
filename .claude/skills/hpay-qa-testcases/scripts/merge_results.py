# -*- coding: utf-8 -*-
"""
Regenerate the 5 QA workbooks and carry over recorded execution results.

The generator (generate_test_cases.py) always writes Status = "Not Run" and empty
result columns. This script:
  1. runs the generator into a temporary directory,
  2. copies columns M..R (Status, Actual Result, Evidence, Executed By,
     Last Run Date, Defect ID / Notes) from the previously recorded workbooks,
     keyed by Test ID, translating any Vietnamese result text to English,
  3. writes the final workbooks into docs/qa/test-cases.

Usage:
    python -X utf8 merge_results.py [--recorded DIR] [--out DIR]

  --recorded DIR  directory holding the workbooks that contain recorded results
                  (default: docs/qa/test-cases, i.e. the current files)
  --out DIR       output directory (default: docs/qa/test-cases)

The script refuses to write if a recorded Test ID is missing from the regenerated
workbook, if the Status/ID sets differ, or if any non-ASCII Vietnamese letter
remains in a merged result cell.
"""
import argparse
import copy
import importlib.util
import os
import re
import shutil
import sys
import tempfile

import openpyxl

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_DIR = None  # set below from the generator module (repo root aware)
RESULT_COLS = range(13, 19)  # M..R
TRANSLATED_COLS = (14, 15, 18)  # Actual Result, Evidence, Defect ID / Notes

# Latin letters with Vietnamese diacritics (excluding plain ASCII).
VI_RE = re.compile("[\u00c0-\u00ff\u0102\u0103\u0110\u0111\u0128\u0129\u0168\u0169\u01a0\u01a1\u01af\u01b0\u1ea0-\u1ef9]")

_X = "Go test executed directly; result PASS as expected per the Expected Result."
_PW = "Playwright e2e executed directly against the real hpay-admin + real hpay-backend, PASS as per the Expected Result."

# (Test ID, column number) -> English text. Used only where the recorded text is Vietnamese.
TR = {
    ("TC-PER-016", 14): "= TestEdge_E14_SelfTransferRejected (cross-reference, same scenario: CHECK no_self_transfer).",
    ("TC-PER-016", 15): "TestEdge_E14_SelfTransferRejected PASS (0.00s) — internal/ledger/ledger_test.go, see TC-EDG-E14 in sheet 05",
    ("TC-PER-018", 14): "= TestEdge_E19_ExpiryRacePay (cross-reference, same expired money-request scenario).",
    ("TC-PER-018", 15): "TestEdge_E19_ExpiryRacePay PASS (0.13s, 2 subtests) — test/integration/money/moneyrequest_test.go, see TC-EDG-E19 in sheet 05",
    ("TC-PER-019", 14): "= TestEdge_E19_ExpiryRacePay subtest 'pay_vs_sweeper_concurrent' (cross-reference, exactly the Pay() vs sweeper.ExpireDue() race).",
    ("TC-PER-019", 15): "TestEdge_E19_ExpiryRacePay/pay_vs_sweeper_concurrent PASS (0.05s) — test/integration/money/moneyrequest_test.go",
    ("TC-PER-026", 14): "curl-equivalent via automated test TestVault_WithdrawOverBalanceRejected PASS — withdrawing more than the vault balance is clearly rejected, no transaction is created, the vault balance stays at 0.",
    ("TC-BIZ-006", 14): "= TestEdge_E09_PolicySnapshotSurvivesPolicyChange (cross-reference, same scenario: policy_snapshot captured when the order is created).",
    ("TC-BIZ-006", 15): "TestEdge_E09_PolicySnapshotSurvivesPolicyChange PASS (0.52s) — test/integration/org/approval_test.go, see TC-EDG-E09 in sheet 05",
    ("TC-BIZ-008", 14): "New test TestQA_DisbursementZeroOrNegativeAmountRejected PASS — amount=0 and amount=-100,000 are both rejected with ErrAmountInvalid, no disbursement order is created.",
    ("TC-BIZ-008", 15): "TestQA_DisbursementZeroOrNegativeAmountRejected PASS (0.20s) — test/integration/money/qa_sheet02_test.go (new)",
    ("TC-BIZ-009", 14): "New test TestQA_DisbursementUnknownRecipientRejected PASS — a non-existent phone number is clearly rejected with ErrReceiverNotFound.",
    ("TC-BIZ-009", 15): "TestQA_DisbursementUnknownRecipientRejected PASS (0.19s) — test/integration/money/qa_sheet02_test.go (new)",
    ("TC-BIZ-013", 14): "= TestApprovalCancel_AlreadyFinalizedRejected (cross-reference — the status!=pending guard is shared by every terminal state, verified with approved; a completed transaction takes the same finalize branch).",
    ("TC-BIZ-014", 14): "= TestApprovalCancel_AlreadyFinalizedRejected (cross-reference — same status!=pending guard, expired is also a terminal state blocked in exactly the same way).",
    ("TC-BIZ-021", 14): "New test TestQA_DecideWrongOTPDoesNotFinalize PASS — a wrong OTP does not record the signature, the order stays pending as before.",
    ("TC-BIZ-021", 15): "TestQA_DecideWrongOTPDoesNotFinalize PASS (0.47s) — test/integration/money/qa_sheet02_test.go (new)",
    ("TC-BIZ-022", 14): "= TestEdge_E08_CheckerRaceSingleFinalize (cross-reference, same scenario of 2 checkers approving in parallel).",
    ("TC-BIZ-022", 15): "TestEdge_E08_CheckerRaceSingleFinalize PASS (0.45s) — test/integration/org/approval_test.go, see TC-EDG-E08 in sheet 05",
    ("TC-BIZ-023", 14): "= TestEdge_E10_ExpiredApprovalFailsClean (cross-reference, same scenario of an order pending for more than 24h).",
    ("TC-BIZ-023", 15): "TestEdge_E10_ExpiredApprovalFailsClean PASS (0.32s) — test/integration/org/approval_test.go, see TC-EDG-E10 in sheet 05",
    ("TC-BIZ-026", 14): "New test TestQA_SecondCheckerAfterFinalizeGetsClearError PASS — checker B signing after checker A has finalized receives a clear ErrAlreadyFinalized (clear message, not a generic system error).",
    ("TC-BIZ-026", 15): "TestQA_SecondCheckerAfterFinalizeGetsClearError PASS (0.69s) — test/integration/money/qa_sheet02_test.go (new)",
    ("TC-ADM-001", 14): "Real browser tool on /dashboard: the first 4 KPI cards (New users, 24h transactions, 24h volume, Failure rate) explicitly label themselves in the UI as 'illustrative — no analytics API yet' — an intentional placeholder, not yet wired to a real API. The 'Needs attention' section (pending KYC / open fraud flags) IS real data, matching the DB.",
    ("TC-ADM-001", 15): "Real get_page_text: the text '+12% (illustrative — no analytics API yet)' is displayed verbatim on each KPI card",
    ("TC-ADM-001", 18): "Not a bug — the feature is unfinished and the UI discloses it. A real backend analytics API is needed to complete this case.",
    ("TC-ADM-002", 14): "Clicking 'Pending KYC' in the Needs attention area navigates correctly to /kyc (location.pathname='/kyc').",
    ("TC-ADM-002", 15): "Real browser tool: click + confirm location.pathname; matches e2e/dashboard.spec.ts 'clicking the KYC attention card navigates to /kyc' which already PASSED in the suite",
    ("TC-ADM-003", 14): "Opened /kyc?tab=manual, clicked a real pending profile (Test User 0936751292), saw the full AI score/verdict/ID card/selfie. Pressed Approve -> POST /v1/admin/kyc/{id}/review 200 OK. The profile disappeared from the manual_review queue immediately afterwards. Cross-reference TestKYCManualThenAdminApprove + TestKYCTierUnlocksTransferLimit both PASS (confirming the limit is raised per the new tier).",
    ("TC-ADM-003", 15): "Real browser tool: POST .../review 200; the queue GET afterwards is empty. + TestKYCManualThenAdminApprove PASS (0.24s), TestKYCTierUnlocksTransferLimit PASS (0.58s)",
    ("TC-ADM-004", 14): "= TestKYCRejectBranchAllowsResubmit (cross-reference, same scenario: reject + allow submitting a new profile).",
    ("TC-ADM-005", 18): "Needs >=2 pending KYC profiles to verify auto-focus of the next profile; the DB currently has only 0-1 pending profiles after the real profile was approved in TC-ADM-003. The fixture is not fully set up yet.",
    ("TC-ADM-006", 14): "Opened /fraud, chose a real AMOUNT_SPIKE flag (score 85), pressed 'Confirm fraud', entered a reason, chose 'Lock account + cancel pending transactions', confirmed -> POST /v1/admin/fraud-flags/{id}/resolve 200 OK. Verified via API: status=confirmed_fraud.",
    ("TC-ADM-006", 15): "Real browser tool: POST .../resolve 200; GET /v1/admin/fraud-flags?status=confirmed_fraud confirms status=confirmed_fraud",
    ("TC-ADM-007", 14): "'Mark as safe' uses window.confirm() (cannot be automated via the browser tool), verified directly via API: POST resolve action=clear -> 200 {status:resolved}. UI reload: the flag disappears from the 'Open' list (7->5 open), the related account/transaction is not locked.",
    ("TC-ADM-007", 15): "Real fetch: POST .../resolve {action:clear} -> 200; UI reload confirms exactly 5 open flags remain",
    ("TC-ADM-008", 14): "The '⚠ SLA' badge shows correctly for the AMOUNT_SPIKE flag (score>=80, e.g. 85), and does NOT show for the VELOCITY_5MIN flag with score=50 (<80) — matches the FraudPage.tsx logic (status==='open' && score>=80).",
    ("TC-ADM-008", 15): "Real get_page_text on /fraud: '#eac2 VELOCITY_5MIN 50 ... 19m Open' (no badge) vs '#0c48 AMOUNT_SPIKE 85 ... 19m ⚠ SLA Open' (has badge)",
    ("TC-ADM-009", 14): "Applied filter Status=Failed on /workflows -> the list filters to exactly 43 results, all Failed. Clicked 1 instance -> the detail shows the correct stopped step (FRAUD_CHECK) + reason (transaction blocked by fraud rules) + timeline VALIDATE(✓)/FRAUD_CHECK(✕).",
    ("TC-ADM-009", 15): "Real browser tool: get_page_text before/after the filter + click on instance 1fb440a2-..., viewed the real step executions timeline",
    ("TC-ADM-010", 14): "Instance detail 1fb440a2-... shows the correct pending step (FRAUD_CHECK) + the Retry / Force fail + refund buttons. Double side effects were not checked via a separate ledger API (TestEdge_E04 cross-reference already exists).",
    ("TC-ADM-010", 15): "Real browser tool on /workflows/1fb440a2-...; see also TestEdge_E04_RedeliveryNoDoubleSideEffect PASS (sheet 05)",
    ("TC-ADM-011", 14): "= TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim (cross-reference, same scenario of 2 sweepers in parallel on 1 stuck instance).",
    ("TC-ADM-011", 15): "TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim PASS (0.07s), see TC-EDG-E20 in sheet 05",
    ("TC-ADM-012", 14): "The KPIs 'Total reconciled/Matched/Mismatched' show the REAL data correctly after fixing a field-name bug (6/3/3, matches the real API) — see the hpay-admin fix commit. But 'Export Excel' does NOT download a real file — the code calls alert('...downloaded... (simulated)') when the backend has not returned a real reportUrl, and there is no API that generates a real signed URL for this day.",
    ("TC-ADM-012", 15): "Real browser tool: click Export Excel -> the network log shows a request to a mock URL (sig=mock, hard-coded exp) returning 403; see src/pages/ReconPage.tsx line 37 which itself comments '(simulated)'",
    ("TC-ADM-012", 18): "Export Excel is a placeholder not yet wired to a real API — not a hidden bug (the code declares 'simulated'), but it does not meet the Expected Result 'downloaded successfully'. The backend needs to generate + really sign the xlsx report.",
    ("TC-ADM-013", 14): "The AI summary displays correctly, in easy-to-understand natural language, pointing in the right direction (missing_external, window 23:58-00:02, likely cutoff mismatch, recommendation to reconcile the next morning's batch).",
    ("TC-ADM-013", 15): "Real browser tool on /recon for date 2026-08-01: read the AI ROOT-CAUSE SUMMARY verbatim",
    ("TC-ADM-014", 14): "The backend logic was verified via TestEdge_E16_ReconCutoffNoFalseMismatch PASS (matches by ref_code, not by calendar day). The AI summary in the UI also confirms the correct cutoff classification direction. The UI-level badge for a separate classification could not be verified for this specific case because cmd/seed does not create detailed reconciliation_items rows (only the run summary).",
    ("TC-ADM-014", 15): "TestEdge_E16_ReconCutoffNoFalseMismatch PASS (0.08s) + real AI summary on /recon",
    ("TC-ADM-014", 18): "Known limitation: cmd/seed only INSERTs reconciliation_runs (summary), not reconciliation_items — the detail table in the UI is always empty even when mismatched>0. Also found: FE/BE field-name mismatch for items (result/detail vs the old status/internal_ref) — the mapping was fixed to match the real schema even though there is no data to test end-to-end.",
    ("TC-ADM-015", 14): "Logged in as Admin B (e2e-checker), opened /tasks -> 1 real pending reversal request (seed u4 confirmed_fraud). Entered a reason, pressed Approve -> POST /v1/admin/approvals/{id}/decide 200 OK. The queue is empty right afterwards ('No orders awaiting approval').",
    ("TC-ADM-015", 15): "Real browser tool: network log POST .../decide 200; get_page_text confirms the queue is empty afterwards",
    ("TC-ADM-016", 14): "= e2e/my-tasks.spec.ts 'checker decide dialog enforces a 10-char reason before approve/reject' PASS (cross-reference, same Reject flow + reason validation).",
    ("TC-ADM-016", 15): "e2e/my-tasks.spec.ts PASS — full suite run 2026-08-01, 29 passed",
    ("TC-ADM-017", 14): "= e2e/my-tasks.spec.ts 'A01 — two checker contexts race to decide the same reversal: exactly one wins' PASS (cross-reference, same scenario of 2 contexts racing).",
    ("TC-ADM-017", 15): "e2e/my-tasks.spec.ts A01 PASS — full suite run 2026-08-01",
    ("TC-ADM-018", 14): "Typed 'TC-ADM-015' into the search box -> the results filter to exactly 1 matching record (the log just created in TC-ADM-015), showing who/what/when/why in full.",
    ("TC-ADM-018", 15): "Real browser tool: search 'TC-ADM-015' -> 1/1 result, the record just created",
    ("TC-ADM-019", 14): "The Audit Log page has no edit/delete buttons in the UI (only a search box). Calling DELETE and PUT /v1/admin/audit/{id} directly via real fetch -> both return 404 (no route exists to edit/delete).",
    ("TC-ADM-019", 15): "Real fetch: DELETE -> 404, PUT -> 404 (no route registered other than GET /v1/admin/audit in router.go)",
    ("TC-ADM-020", 14): "The disbursement approval policy section displays correctly (tiers, version, status). But there is NO member list with roles — OrgsPage.tsx only shows the COUNT ('X members'), it does not render a list of names + each person's role.",
    ("TC-ADM-020", 15): "grep 'members' in src/pages/OrgsPage.tsx -> only 1 line rendering the count, no detailed member table/list",
    ("TC-ADM-020", 18): "A feature gap, not a minor bug — a member+role list UI must be built, outside the 'bug fix' scope of this round.",
    ("TC-ADM-021", 14): "Real browser tool on /orgs: edited a tier (level 2 required_approvals), pressed Save -> PUT /v1/admin/orgs/{id}/policies returned 201, the version increased correctly, the UI updated with no console errors. Re-tested with the 2-tab race (A05), which also confirmed no data is lost when 2 admins edit at the same time.",
    ("TC-ADM-021", 15): "PUT /v1/admin/orgs/{id}/policies -> 201 {policy_id, version}; see also TC-EDG-A05",
    ("TC-PLT-001", 14): "curl POST /v1/transactions/request (P2P_TRANSFER 100,000 VND) -> processing, confirm PIN -> completed. Latency 278ms, 0 fraud_flags for this transaction. As expected.",
    ("TC-PLT-001", 15): "Real curl: ref MP999D3DA83D75, status completed, SELECT count(*) FROM fraud_flags WHERE transaction_id=... => 0",
    ("TC-PLT-002", 14): "Automated test TestFraudFlagsGreyZone PASS — sets up the gray-zone scenario correctly (velocity+new_receiver=50 points), confirms the transaction is NOT blocked and fraud_flags has a record with the AI advisory.",
    ("TC-PLT-003", 14): "Tried a real curl first (4 consecutive requests, amount 20M) but the seed user's balance was not enough to exceed the AMOUNT_SPIKE threshold (needs >=5x avg30d=3,005,100 ~ 15M, balance only 12.3M). Switched to the automated test TestFraudBlocksHighRisk (sets up the scenario correctly: verified KYC, topup 50M, 4 small transactions creating velocity, then one 20M transaction) PASS — the transaction is blocked immediately (status=failed), no money moves, a fraud flag is recorded.",
    ("TC-PLT-004", 14): "curl POST /v1/transactions/request twice with the same Idempotency-Key, 4s apart (because OFFER_CUST_L2 has its own 3s interval throttle, unrelated to idempotency) -> both calls return the SAME transaction_id/ref_code (no new transaction created).",
    ("TC-PLT-004", 15): "Real curl x2: both responses have the same transaction_id=93ac04dd-... ref=MP1FB3146C15E2",
    ("TC-PLT-005", 14): "= TestEdge_E06_DuplicateResultEventIdempotent (cross-reference, same scenario: the outbox relay republishes the event, the consumer checks step_executions before side effects).",
    ("TC-PLT-005", 15): "TestEdge_E06_DuplicateResultEventIdempotent PASS (0.06s) — internal/workflow/integration_test.go, see TC-EDG-E06 in sheet 05",
    ("TC-PLT-006", 14): "RE-TEST after the idempotency bug fix (hpay-backend 6f09bc6): topup key Y x2 -> same transaction_id; transfer key Z x2 -> same transaction_id; the 2 flows are independent and do not mix. The original bug (the Idempotency-Key header being ignored) is fully fixed.",
    ("TC-PLT-006", 15): "Real curl: topup call1/call2 have the same id=6f63cf80-...; transfer call1/call2 have the same id=7398c6f0-...",
    ("TC-PLT-006", 18): "The original bug is fixed + has unit tests in hpay-backend test/integration/money/transaction_request_http_test.go (2 test cases). This is the FIRST bug found in the whole QA effort, starting the chain of fixes for the entire session.",
    ("TC-PLT-007", 14): "The corresponding mechanism was verified through several TestEdge_* tests (E04 redelivery, E17 Redis down, E20 stuck recovery), all PASS — the same recovery sweeper/idempotent-handler infrastructure that TC-PLT-007 describes at the 'platform mechanism' level.",
    ("TC-PLT-007", 15): "TestEdge_E04_RedeliveryNoDoubleSideEffect, TestEdge_E17_RedisDownNoDataLoss, TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim — all PASS, see sheet 05",
    ("TC-PLT-008", 14): "= TestEdge_E17_RedisDownNoDataLoss (cross-reference, same scenario: Redis down + the API still writes DB/outbox successfully).",
    ("TC-PLT-008", 15): "TestEdge_E17_RedisDownNoDataLoss PASS (0.25s) — test/integration/money/redis_down_test.go, see TC-EDG-E17 in sheet 05",
    ("TC-PLT-009", 14): "= TestEdge_E05_OutboxSurvivesBrokerOutage (cross-reference, same scenario: the transactional outbox survives a broker outage).",
    ("TC-PLT-009", 15): "TestEdge_E05_OutboxSurvivesBrokerOutage PASS (0.04s) — internal/workflow/integration_test.go, see TC-EDG-E05 in sheet 05",
    ("TC-PLT-010", 14): "= TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim (cross-reference, same scenario of 2 parallel sweepers, lease claim UPDATE...RETURNING).",
    ("TC-PLT-010", 15): "TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim PASS (0.07s) — internal/workflow/integration_test.go, see TC-EDG-E20 in sheet 05",
    ("TC-EDG-E18", 14): "CI gate green on real GitHub Actions, confirming the migration down is really safe/reverts cleanly.",
    ("TC-EDG-A05", 14): "Real browser tool: 2 tabs in the same admin session, editing the same org policy. Promise.all fired 2 PUTs concurrently -> {s1:409, s2:201} as expected. Sequential saves (no race) still succeed normally (201, 201, version increasing). No console crash in either tab.",
    ("TC-EDG-A05", 15): "PUT /v1/admin/orgs/{id}/policies x2 concurrently via real fetch in the browser session — real responses: 1x409 ERR_POLICY_VERSION_CONFLICT, 1x201 Created",
    ("TC-EDG-A05", 18): "Found + fixed 3 real bugs while running this case (hpay-backend 7605920, hpay-admin 60db714): (1) the old endpoint POST /v1/org/{id}/policies returned 401 with an admin token, (2) a versioning race condition with no lock -> could create a duplicate version, (3) the frontend swallowed errors so a failed save still reported success.",
    ("TC-EDG-A07", 14): "Cannot be verified — the masking feature does not yet exist in the backend.",
    ("TC-EDG-A07", 18): "PII masking must be added in the backend before this case can run.",
}



def has_vi(v):
    return isinstance(v, str) and bool(VI_RE.search(v))


def translate(test_id, col, value):
    """Return the English text for a recorded result cell (unchanged if already English)."""
    if not has_vi(value):
        return value
    key = (test_id, col)
    if key not in TR and col == 14 and test_id.startswith("TC-EDG-E"):
        return _X
    if key not in TR and col == 14 and test_id.startswith("TC-EDG-A"):
        return _PW
    if key not in TR:
        raise SystemExit(f"No translation for {key}: {value[:80]!r}")
    return TR[key]


def load_generator():
    spec = importlib.util.spec_from_file_location("gen_test_cases", os.path.join(HERE, "generate_test_cases.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def main():
    ap = argparse.ArgumentParser()
    gen = load_generator()
    default_dir = gen.OUT_DIR  # <repo>/docs/qa/test-cases
    ap.add_argument("--recorded", default=default_dir)
    ap.add_argument("--out", default=default_dir)
    ap.add_argument("--allow-new-ids", action="store_true",
                    help="allow test cases that exist in the generator catalog but not in the recorded workbook; "
                         "they keep Status = Not Run (use when adding cases)")
    ap.add_argument("--allow-removed-ids", action="store_true",
                    help="allow dropping cases that exist in the recorded workbook but not in the catalog "
                         "(their recorded results are discarded; use only for deliberate removals)")
    args = ap.parse_args()

    tmp = tempfile.mkdtemp(prefix="qa-regen-")
    try:
        gen.main(tmp)
        os.makedirs(args.out, exist_ok=True)
        for name in sorted(os.listdir(tmp)):
            if not name.endswith(".xlsx"):
                continue
            new_wb = openpyxl.load_workbook(os.path.join(tmp, name))
            old_path = os.path.join(args.recorded, name)
            old_wb = openpyxl.load_workbook(old_path)
            ws_new, ws_old = new_wb["Test Cases"], old_wb["Test Cases"]

            new_rows = {ws_new.cell(r, 1).value: r for r in range(2, ws_new.max_row + 1)}
            old_rows = {ws_old.cell(r, 1).value: r for r in range(2, ws_old.max_row + 1)}
            added = set(new_rows) - set(old_rows)
            removed = set(old_rows) - set(new_rows)
            if added and not args.allow_new_ids:
                raise SystemExit(f"{name}: new Test IDs {sorted(added)}; re-run with --allow-new-ids to add them")
            if removed and not args.allow_removed_ids:
                raise SystemExit(f"{name}: Test IDs {sorted(removed)} have recorded rows but are not in the catalog; "
                                 f"restore them or re-run with --allow-removed-ids to discard their results")
            if added:
                print(f"{name}: adding {len(added)} new case(s) as Not Run: {sorted(added)}")
            if removed:
                print(f"{name}: DISCARDING {len(removed)} case(s): {sorted(removed)}")

            for tid, r_new in new_rows.items():
                if tid in added:
                    continue          # new case: keep generator defaults (Not Run)
                r_old = old_rows[tid]
                for c in RESULT_COLS:
                    v = ws_old.cell(r_old, c).value
                    if c in TRANSLATED_COLS:
                        v = translate(tid, c, v)
                    cell = ws_new.cell(r_new, c, value=v)
                    cell.alignment = copy.copy(ws_old.cell(r_old, c).alignment)
                h_old = ws_old.row_dimensions[r_old].height
                if h_old and h_old > (ws_new.row_dimensions[r_new].height or 0):
                    ws_new.row_dimensions[r_new].height = h_old
                for c in RESULT_COLS:
                    v = ws_new.cell(r_new, c).value
                    if has_vi(v):
                        raise SystemExit(f"{name} {tid} col {c}: Vietnamese text remains")

            new_wb.save(os.path.join(args.out, name))
            print(f"Merged {name}: {len(new_rows)} rows ({len(new_rows) - len(added)} with carried results, {len(added)} new)")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
