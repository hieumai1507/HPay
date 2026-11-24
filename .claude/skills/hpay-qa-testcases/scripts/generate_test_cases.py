# -*- coding: utf-8 -*-
"""
Generate the 5 manual-QA Excel test-case workbooks for HPay.

Source of truth: .claude/skills/hpay-qa-testcases/references/test-case-schema.md (schema, ID
convention, executor rules) and references/qa-architecture.md.
Content sourced from:
  - docs/product/feature-catalog.md (FEATURES array — 28 features)
  - docs/history/design-v2/01-backend-solution-architecture.md (E01-E20)
  - docs/history/design-v2/02-mobile-solution-architecture.md (M01-M12)
  - docs/history/plans/admin/e2e-playwright-suite/implementation-plan.md §8 (A01-A07)

Re-run with: python -X utf8 .claude/skills/hpay-qa-testcases/scripts/generate_test_cases.py
Output: <repo>/docs/qa/test-cases/0{1..5}-*.xlsx

NOTE: this generator always writes Status = "Not Run" and empty result columns.
To carry recorded execution results over, run merge_results.py afterwards.
"""
import os
from openpyxl import Workbook
from openpyxl.styles import Font, Alignment, PatternFill, Border, Side
from openpyxl.utils import get_column_letter
from openpyxl.worksheet.datavalidation import DataValidation

def _repo_root():
    d = os.path.dirname(os.path.abspath(__file__))
    while True:
        if os.path.exists(os.path.join(d, "CLAUDE.md")) and os.path.isdir(os.path.join(d, "docs")):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            raise RuntimeError("repo root (CLAUDE.md + docs/) not found above " + __file__)
        d = parent


REPO_ROOT = _repo_root()
OUT_DIR = os.path.join(REPO_ROOT, "docs", "qa", "test-cases")

HEADERS = [
    "Test ID", "Feature", "Role / Actor", "Test Type", "Priority",
    "Precondition / Setup", "Test Steps", "Test Data", "Expected Result",
    "Related Edge ID", "Executor", "Execution Method", "Status",
    "Actual Result", "Evidence", "Executed By", "Last Run Date", "Defect ID / Notes",
]

COL_WIDTHS = [14, 26, 16, 14, 8, 30, 40, 24, 36, 12, 11, 30, 10, 26, 20, 14, 13, 22]

TEST_TYPES = ["Happy Path", "Negative", "Edge Case", "Boundary", "Security/AuthZ"]
PRIORITIES = ["P0", "P1", "P2"]
STATUSES = ["Not Run", "Pass", "Fail", "Blocked", "N/A"]
EXECUTORS = ["AUTO-API", "AUTO-WEB", "AUTO-UI", "PARTIAL", "HUMAN"]

HEADER_FILL = PatternFill("solid", fgColor="0B5FFF")
HEADER_FONT = Font(bold=True, color="FFFFFF", size=10)
WRAP = Alignment(wrap_text=True, vertical="top")
THIN = Side(style="thin", color="E5E7EB")
BORDER = Border(left=THIN, right=THIN, top=THIN, bottom=THIN)


# ---------------------------------------------------------------------------
# Edge case reference (37 IDs, verified against real TestEdge_*/TestE2E_* names
# in hpay-backend / hpay-mobile source, and hpay-admin's E2E plan §8 for A-series)
# ---------------------------------------------------------------------------
EDGE_CASES = {
    "E01": ("Client sends a duplicate top-up/transfer request (double-tap / network retry)",
             "hpay-backend", "TestEdge_E01_IdempotentTopup",
             "Idempotency-Key UNIQUE — the 2nd request with the same key returns the original result and creates no new transaction"),
    "E02": ("2 concurrent transactions withdraw from the same wallet, total exceeds the balance",
             "hpay-backend", "TestEdge_E02_ConcurrentOverdraw",
             "SELECT FOR UPDATE serializes them — exactly 1 transaction succeeds, the other is rejected for insufficient balance"),
    "E03": ("A→B and B→A at the same time (2 wallets locking each other crosswise)",
             "hpay-backend", "TestEdge_E03_CrossTransferNoDeadlock",
             "Lock ordering by wallet_id — both transactions complete, no deadlock, no timeout"),
    "E04": ("Worker dies midway after DEBIT is done but before CREDIT",
             "hpay-backend", "TestEdge_E04_RedeliveryNoDoubleSideEffect",
             "Redis Streams message not yet XACKed → XAUTOCLAIM hands it to another worker; handler is idempotent (checks step_executions) — no duplicate DEBIT/CREDIT"),
    "E05": ("DB commit succeeds but event publish fails",
             "hpay-backend", "TestEdge_E05_OutboxSurvivesBrokerOutage",
             "Transactional outbox — the event stays in the DB, the outbox relay republishes it when the broker recovers, no event is lost"),
    "E06": ("Event is published twice (relay retry)",
             "hpay-backend", "TestEdge_E06_DuplicateResultEventIdempotent",
             "Consumer checks step_executions (instance, step, attempt) before side effects — the 2nd event is skipped, no double processing"),
    "E07": ("CREDIT fails permanently after DEBIT succeeded (receiving wallet locked midway)",
             "hpay-backend", "TestEdge_E07_CompensationReversesMovedFunds",
             "Saga compensation: post a reversal entry, transaction moves to failed, sender receives a refund notification, sender balance is restored exactly"),
    "E08": ("Checker A and B press approve at the same time on a tier that needs only 1 signature",
             "hpay-backend", "TestEdge_E08_CheckerRaceSingleFinalize",
             "FOR UPDATE on transaction_approvals + UNIQUE(approval_id, checker_id) — exactly 1 checker wins and finalizes; the other receives a message that the order was already processed (not a system error)"),
    "E09": ("Admin edits the approval policy while an order is awaiting approval",
             "hpay-backend", "TestEdge_E09_PolicySnapshotSurvivesPolicyChange",
             "policy_snapshot is captured when the order is created — the pending order continues under the old policy, unaffected by the new policy version"),
    "E10": ("Organization disbursement order waits for approval for more than 24h",
             "hpay-backend", "TestEdge_E10_ExpiredApprovalFailsClean",
             "expires_at + sweeping cron → approval becomes expired, workflow fails cleanly, no funds are left dangling"),
    "E11": ("OTP entered incorrectly 3 times in a row",
             "hpay-backend", "TestEdge_E11_OTPBruteForce",
             "attempts >= max_attempts → the current challenge is invalidated and a new OTP challenge must be requested; 5 consecutive failed challenges → approve is locked for 15 minutes"),
    "E12": ("User is locked (fraud) exactly while a transaction is mid-workflow",
             "hpay-backend", "TestEdge_E12_LockedUserFailsMidWorkflow",
             "Every wallet step re-checks account_status inside the same transaction even though the VALIDATE step already passed — the transaction is blocked midway, no funds move after the lock"),
    "E13": ("Transaction amount is zero, negative, or exceeds the limit for the KYC tier",
             "hpay-backend", "TestEdge_E13_LimitByKycTier",
             "Validator + CHECK constraint block it at both the API and DB layers — the request is clearly rejected, no transaction is created"),
    "E14": ("Transfer to oneself (self-transfer)",
             "hpay-backend", "TestEdge_E14_SelfTransferRejected",
             "CHECK no_self_transfer + API validation — the request is rejected before touching the workflow engine"),
    "E15": ("Top-up: the (mock) bank callback arrives twice for the same top-up",
             "hpay-backend", "TestEdge_E15_DuplicateBankCallback",
             "Idempotent by statement ref_code — the 2nd callback is skipped, the wallet is credited exactly once"),
    "E16": ("Clock skew / transaction straddling midnight during reconciliation",
             "hpay-backend", "TestEdge_E16_ReconCutoffNoFalseMismatch",
             "Recon matches by ref_code, not only by date — a transaction across the cutoff is classified separately, no false mismatch is reported"),
    "E17": ("Redis is completely down while new transactions arrive",
             "hpay-backend", "TestEdge_E17_RedisDownNoDataLoss",
             "The create-transaction API still writes DB + outbox successfully (202); the engine pauses progress until Redis recovers, no data loss, no 500 error"),
    "E18": ("A new migration breaks the existing schema",
             "hpay-backend", "CI job: migrate up + down + up on a clean DB",
             "CI blocks the merge if migrate down does not revert cleanly or the 2nd migrate up fails — no broken migration reaches main"),
    "E19": ("Money request expires exactly when the payer presses \"Pay\" (race with the sweeper)",
             "hpay-backend", "TestEdge_E19_ExpiryRacePay",
             "FOR UPDATE serializes Pay() and sweeper.ExpireDue() — exactly 1 of the 2 wins; if expiry wins, Pay() returns ErrExpired and no \"orphan\" transaction is created"),
    "E20": ("2 sweepers (recovery) run in parallel on the same stuck workflow instance",
             "hpay-backend", "TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim",
             "Lease claim via UPDATE ... SET status='recovering' WHERE ... RETURNING — only 1 sweeper claims successfully, the event is re-emitted only once, the instance is not recovered twice"),

    "M01": ("User double-taps the \"Transfer\" button",
             "hpay-mobile", "TestEdge_M01_DoubleTapTransfer_SingleRequest",
             "The button is disabled immediately on tap + the Idempotency-Key is generated exactly once when entering the confirm screen (not regenerated on press) — only 1 transaction is created no matter how many taps"),
    "M02": ("Request times out / network is lost right after pressing transfer",
             "hpay-mobile", "TestEdge_M02_TransferPollTimeout_ProcessingCopy",
             "Do NOT show \"failed\" prematurely — switch to the Processing screen and poll GET /transactions/{refCode} using the same idempotent retry; after a 30s poll timeout show \"Processing, check again in History\""),
    "M03": ("Transaction is async, workflow has not finished processing",
             "hpay-mobile", "TestEdge_M03_TransferFailed_ErrorMapDisplay",
             "The Processing screen polls with backoff 1s→2s→4s; the pending_approval state shows dedicated copy \"Awaiting business approval\" instead of a generic error"),
    "M04": ("Access token expires mid-operation (midway through a flow)",
             "hpay-mobile", "TestEdge_M04_Refresh_SingleFlight_RetryOnce",
             "The interceptor refreshes automatically in single-flight mode (only 1 refresh request, other requests queue and wait); if refresh fails → navigate to Login but KEEP the form draft being entered"),
    "M05": ("Refresh token is revoked (login on another device / theft detection)",
             "hpay-mobile", "TestEdge_M05_RefreshRevoked_ForceLogout",
             "Force logout immediately + a clear reason is shown (not a generic network error) — all sessions on the old device are revoked"),
    "M06": ("Entering a large amount / unusual format in the input field",
             "hpay-mobile", "TestEdge_M06_BigIntMoney_NoPrecisionLoss",
             "Accepts integer VND only, thousands separators formatted while typing, unusual characters blocked on paste; parsed via string→BigInt — never passes through a JS float, no precision loss for large amounts"),
    "M07": ("App goes to the background while on the OTP/confirm screen",
             "hpay-mobile", "TestEdge_M07_AppBackground_PrivacyOverlay",
             "All app content is blurred with a Native Blur when entering the Task Switcher (like VCB Digibank); an OTP still within its TTL is kept, an expired one prompts for a new one when returning to the app"),
    "M08": ("\"Approve disbursement\" deep link opened while the user is not logged in",
             "hpay-mobile", "TestEdge_M08_DeepLink_PendingAuth",
             "The pending route is saved to the store before the Login screen; after a successful login, navigate straight to the approval screen that was clicked from the deep link"),
    "M09": ("Checker opens an order that someone else already approved/rejected",
             "hpay-mobile", "TestEdge_M09 (described in 02-mobile-solution-architecture.html — no dedicated unit test yet, verified through the real UI)",
             "The UI handles ERR_ALREADY_FINALIZED: automatically refreshes the inbox, shows a toast with the order's actual new state, does not crash and does not show a generic network error"),
    "M10": ("Captured ID card/selfie photo is blurry or too dark",
             "hpay-mobile", "TestEdge_M10_KYCImageHeuristic_Validation",
             "Client-side check (blur/brightness heuristic) before upload, guides the user to retake right there — reduces the loop of AI/staff rejections after the upload"),
    "M11": ("KYC photo upload loses the network midway",
             "hpay-mobile", "TestEdge_M11_KYCUpload_NetworkDropResume",
             "The 2 photos (document + selfie) are uploaded as separate parts, resumed by request id when the network returns; draft state is saved locally, no need to retake from the start"),
    "M12": ("Long transaction history list, scrolling continuously",
             "hpay-mobile", "TestEdge_M12_HistoryFlashListPagination",
             "Cursor pagination + FlashList; pull-to-refresh does not make the scroll position jump around, no duplicate/repeated items are loaded"),

    "A01": ("2 admins (or 2 checkers) press \"Approve\" on the same task in My Tasks almost simultaneously",
             "hpay-admin", "TestE2E_MyTasks_TwoCheckersRaceOneWins (planned in e2e-playwright-suite, not yet implemented)",
             "1 context receives 200 + the row updates successfully; the other context receives a 409 ERR_ALREADY_FINALIZED error with a clear toast, and the row updates in place to the new state — without reloading the whole table"),
    "A02": ("A new instance is inserted while the reviewer is on a later page (Workflow Monitor)",
             "hpay-admin", "TestE2E_Workflow_PaginationStableUnderInsert (planned in e2e-playwright-suite, not yet implemented)",
             "Cursor pagination (not offset) — next/prev does not shift the rows being viewed even when new workflow instances are created concurrently"),
    "A03": ("Admin session (JWT) expires during a review, with filter/search params in the URL",
             "hpay-admin", "TestE2E_SessionExpiry_ReLoginKeepsFilters (planned in e2e-playwright-suite, not yet implemented)",
             "The next action after the token expires shows a re-login modal (does not bounce to a blank page); after logging in again, return to the same URL + the old filter/search params, without losing the viewing context"),
    "A04": ("KYC image URL (signed URL) expires while the admin is viewing a profile's details",
             "hpay-admin", "TestE2E_KYCDetail_ImageURLExpiryRetry (planned in e2e-playwright-suite, not yet implemented)",
             "The FE receives a 403 on the image request, automatically requests a new URL exactly once and shows the image again — only reports an error to the user if that retry also fails"),
    "A05": ("2 browser tabs open the policy edit screen of the same organization",
             "hpay-admin", "TestE2E_Orgs_TwoTabsEditPolicy (planned in e2e-playwright-suite, not yet implemented)",
             "The tab that saves later receives a 409 (version conflict), handled by the shared interceptor with a consistent toast, no page crash, and the policy just saved by the other tab is not silently overwritten"),
    "A06": ("Admin exports the reconciliation report to Excel (recon export)",
             "hpay-admin", "TestE2E_Recon_ExportDoesNotBlockUI (planned in e2e-playwright-suite, not yet implemented)",
             "The UI remains interactive while waiting for the export (no full-page spinner blocking the whole screen); the system polls until the download link is ready and only then shows the download button"),
    "A07": ("Admin opens the detail of a workflow step (step detail) whose JSON input/output contains personal data (PII)",
             "hpay-admin", "TestE2E_WorkflowDetail_StepJSONMasked (planned in e2e-playwright-suite, not yet implemented)",
             "Both the network response and the rendered DOM mask (***) sensitive PII fields — raw PII is not exposed via DevTools/response even for a valid admin account"),
}

EDGE_ORDER = (
    [f"E{n:02d}" for n in range(1, 21)]
    + [f"M{n:02d}" for n in range(1, 13)]
    + [f"A{n:02d}" for n in range(1, 8)]
)
assert len(EDGE_ORDER) == 39
assert set(EDGE_ORDER) == set(EDGE_CASES.keys())


def row(test_id, feature, role, ttype, priority, pre, steps, data, expected,
        edge="", executor="PARTIAL", method=""):
    return [
        test_id, feature, role, ttype, priority, pre, steps, data, expected,
        edge, executor, method, "Not Run", "", "", "", "", "",
    ]


# ---------------------------------------------------------------------------
# Sheet 01 — 01-personal-user.xlsx  (28 cases, GROUP=PER)
# Executor: PARTIAL by default (mobile UI Claude can't see), HUMAN for camera/biometric capture
# ---------------------------------------------------------------------------
def sheet01_rows():
    R = "Personal User"
    rows = []

    f = "Registration & login"
    rows += [
        row("TC-PER-001", f, R, "Happy Path", "P0",
            "Phone number has never been registered in the system",
            "1. Open the app, choose Register\n2. Enter the phone number + set a password\n3. Confirm the registration OTP\n4. The system issues an access token + refresh token and goes straight to Home",
            "Phone 09xxxxxxx01, password Test@1234",
            "Registration succeeds, a short-TTL access token + long-TTL refresh token are stored, the app enters Home without needing to log in again",
            "", "PARTIAL", "curl POST /v1/auth/register + /v1/auth/login to verify tokens; the mobile UI needs a human to confirm the welcome screen"),
        row("TC-PER-002", f, R, "Negative", "P1",
            "Account already exists with a known password",
            "1. Open the Login screen\n2. Enter the correct phone number with a wrong password once\n3. Observe the error message",
            "Registered phone, wrong password",
            "Login is rejected with a clear message \"wrong phone number or password\", no token is issued, and it does not reveal which field was wrong",
            "", "PARTIAL", "curl POST /v1/auth/login with a wrong password, confirm a 401 response"),
        row("TC-PER-003", f, R, "Edge Case", "P0",
            "Account already exists, on the OTP verification screen (registration or a step that needs OTP)",
            "1. Enter a wrong OTP 3 times in a row\n2. Observe the challenge state\n3. Try entering the old correct OTP the 4th time",
            "Any wrong OTP x3, then the correct OTP of the old challenge",
            "After the 3rd failure the current challenge is invalidated — entering the old correct OTP on the 4th attempt is still rejected, a new OTP must be requested",
            "E11", "PARTIAL", "curl POST /v1/auth/otp/verify 4 times, confirm the response after the 3rd attempt"),
        row("TC-PER-004", f, R, "Edge Case", "P0",
            "User is logged in on device A with a valid refresh token",
            "1. Log in with the same account on device B (simulated with another client)\n2. Go back to device A and try to use the old refresh token to renew the access token",
            "1 account, 2 login sessions (device A, device B)",
            "The system detects the old refresh token being reused after a newer session exists (theft detection) → all sessions on all devices are revoked, device A is force-logged out",
            "M05", "PARTIAL", "curl POST /v1/auth/refresh with device A's token after device B has refreshed — confirm 401 + session revoked"),
    ]

    f = "Identity verification (KYC)"
    rows += [
        row("TC-PER-005", f, R, "Happy Path", "P0",
            "User is registered, KYC not yet verified",
            "1. Open the Identity verification screen\n2. Capture the ID card photo\n3. Capture the selfie for face matching\n4. Wait for the AI (Claude vision) to score",
            "Clear ID card photo + clear selfie, simulated AI score at a high level",
            "The AI returns a high confidence score → auto-approved immediately, the user's daily transaction limit is raised according to the new KYC tier",
            "", "HUMAN", "Needs a real camera to capture the ID card + selfie — cannot be simulated via API/browser tool"),
        row("TC-PER-006", f, R, "Negative", "P1",
            "User is registered, KYC not yet verified",
            "1. Capture ID card/selfie photos of deliberately poor quality (mismatched photos, simulated document)\n2. Wait for the AI to score",
            "Photos below standard / face does not match",
            "The AI returns a low score → auto-rejected, a clear reason is shown, the user is guided to resubmit",
            "", "HUMAN", "Needs a real camera; the backend part (kyc_verification workflow reject) can be verified in parallel via API"),
        row("TC-PER-007", f, R, "Edge Case", "P1",
            "User is on the ID card/selfie capture step",
            "1. Take a photo in low light / with hand shake causing blur\n2. Observe the feedback before the photo is uploaded to the server",
            "Deliberately blurry/dark photo",
            "A client-side heuristic (blur/brightness) blocks it right in the app and asks for a retake — no upload round-trip is wasted on a photo that would certainly be rejected",
            "M10", "HUMAN", "Needs a real camera + real lighting conditions to trigger the heuristic"),
        row("TC-PER-008", f, R, "Edge Case", "P1",
            "User is midway through uploading the 2 KYC photos (document + selfie)",
            "1. Start uploading the document photo\n2. Cut the network midway (turn on airplane mode)\n3. Turn the network back on and observe whether the app resumes by itself",
            "2 valid KYC photos, network cut midway through the first photo upload",
            "Upload is split into parts by request id and resumes exactly from the interrupted point when the network returns; draft state is saved locally, no need to retake from the start",
            "M11", "HUMAN", "Needs a real device to toggle the network during a real upload flow"),
    ]

    f = "Wallet top-up"
    rows += [
        row("TC-PER-009", f, R, "Happy Path", "P0",
            "User is logged in with an active wallet",
            "1. Choose the amount to top up\n2. Authenticate with the OTP sent to the phone\n3. Wait for the (mock) bank to confirm",
            "Amount 500,000 VND, valid mock OTP",
            "The mock bank confirms successfully, the wallet is credited exactly 500,000 VND immediately, with a corresponding double-entry ledger record",
            "", "PARTIAL", "curl POST /v1/wallet/topup + confirm the balance via GET /v1/wallet — the mobile UI needs a human to confirm the animation/notification"),
        row("TC-PER-010", f, R, "Edge Case", "P0",
            "User just pressed top-up, the app hangs/loses the network right after the request is sent (response not received)",
            "1. Send a top-up request with Idempotency-Key X\n2. Simulate network loss/app hang, no response received\n3. User presses top-up again — the app resends the same request with the same key X",
            "Idempotency-Key X, same amount, sent 2 times",
            "The 2nd send with the same Idempotency-Key returns the original transaction result, the wallet is NOT credited twice",
            "E01", "PARTIAL", "curl POST /v1/wallet/topup twice with the same Idempotency-Key header, compare the response + balance before/after"),
        row("TC-PER-011", f, R, "Edge Case", "P0",
            "User has created a top-up request that is waiting for bank confirmation",
            "1. Simulate the (mock) bank callback hitting the system 2 times for the same statement ref_code\n2. Check the wallet balance after both callbacks",
            "1 statement ref_code, call the callback endpoint 2 times in a row",
            "The 2nd callback is recognized as a duplicate by ref_code and skipped — the wallet is credited exactly once",
            "E15", "PARTIAL", "curl POST to the mock bank-callback endpoint twice with the same ref_code, confirm the wallet balance increases only once"),
    ]

    f = "P2P transfer"
    rows += [
        row("TC-PER-012", f, R, "Happy Path", "P0",
            "User A and User B both have active wallets, User A has sufficient balance",
            "1. User A enters User B's phone number + amount\n2. Preview the transaction fee (if any)\n3. Confirm with PIN\n4. Follow the progress screen in real time",
            "Amount 100,000 VND, valid User B phone number, correct PIN",
            "The transaction runs through the workflow engine and completes successfully; User A is debited exactly the amount + fee, User B is credited exactly the amount, both receive notifications",
            "", "PARTIAL", "curl POST /v1/transactions/transfer + confirm both wallet balances via API — the progress-tracking UI needs a human to confirm"),
        row("TC-PER-013", f, R, "Edge Case", "P0",
            "User A has sufficient balance, on the transfer confirmation screen",
            "1. Press the \"Transfer\" button twice quickly in a row (double-tap)\n2. Check the number of transactions created and the balance afterwards",
            "Amount 50,000 VND, double-tap in <1 second",
            "The button is disabled on the first tap + the Idempotency-Key is generated only once for the whole confirm flow — exactly 1 transaction is created, the wallet is debited only once",
            "M01", "PARTIAL", "curl POST /v1/transactions/transfer twice with the same Idempotency-Key to simulate a double-tap at the backend layer"),
        row("TC-PER-014", f, R, "Edge Case", "P0",
            "User A is sending a transfer request",
            "1. Press transfer, simulate network loss right after the request has left the device (response not received)\n2. Observe whether the screen immediately reports \"failed\"\n3. The app automatically polls the transaction status by refCode",
            "Any amount, network timeout simulated right after the request",
            "Do NOT show \"failed\" prematurely — switch to the Processing screen, poll GET /transactions/{refCode}; if polling exceeds 30s without completion, show \"Processing, check again in History\" instead of a wrong error",
            "M02", "PARTIAL", "curl POST transfer then repeated GET /v1/transactions/{refCode} to simulate polling — the Processing UI needs a human to confirm"),
        row("TC-PER-015", f, R, "Edge Case", "P1",
            "User A has a large enough balance (enough to test a high amount)",
            "1. Enter a large amount (e.g. 999,999,999 VND) in the input field\n2. Confirm the displayed number is correct to the last unit, with no rounding/precision loss\n3. Try pasting non-numeric characters into the input field",
            "999,999,999 VND and a string of unusual characters pasted into the input field",
            "The input accepts integer VND only, formats thousands separators while typing, blocks unusual characters on paste; the amount is handled as string→BigInt throughout, never through a JS float, no deviation even for large numbers",
            "M06", "PARTIAL", "curl POST /v1/transactions/transfer with a large minor-unit amount, confirm the response amount matches exactly as a BIGINT"),
        row("TC-PER-016", f, R, "Edge Case", "P0",
            "User A has an active wallet",
            "1. Enter their own phone number as the recipient\n2. Enter any amount\n3. Press confirm",
            "Recipient phone = sender phone",
            "The request is clearly rejected at the API layer (CHECK no_self_transfer), no transaction is created, no money is debited",
            "E14", "AUTO-API", "curl POST /v1/transactions/transfer with recipient_phone = sender_phone, confirm a 400/422 response"),
    ]

    f = "Payment requests & bill splitting"
    rows += [
        row("TC-PER-017", f, R, "Happy Path", "P0",
            "User A and User B both have active wallets",
            "1. User A selects User B, enters the amount + reason\n2. Send the request — User B receives a notification\n3. User B presses \"Pay\" within the 72-hour window\n4. Check the status on both sides",
            "Amount 200,000 VND, reason \"Lunch\"",
            "The request moves to the \"paid\" state after User B pays, the money moves exactly like a normal P2P transaction, User A receives the notification \"B has paid you\"",
            "", "PARTIAL", "curl POST /v1/money-requests + POST /v1/money-requests/{id}/pay, confirm both wallet balances"),
        row("TC-PER-018", f, R, "Negative", "P1",
            "There is a payment request awaiting payment",
            "1. Do nothing until the 72-hour window is exceeded\n2. Check the request status",
            "Request with expires_at set back into the past to simulate expiry",
            "The request automatically moves to the expired state, the payer can no longer press \"Pay\"",
            "", "AUTO-API", "curl UPDATE expires_at into the past (via the test DB tool) then POST pay, confirm the response rejects it as expired"),
        row("TC-PER-019", f, R, "Edge Case", "P0",
            "There is a payment request about to expire, the periodic sweeper is running",
            "1. Set the request's expires_at to exactly the current time (race)\n2. Call Pay() concurrently while letting sweeper.ExpireDue() run\n3. Check the result and the number of transactions created",
            "1 money request, expires_at = now(), 2 flows running in parallel",
            "FOR UPDATE serializes them so exactly 1 of the 2 wins — if expiry wins, Pay() returns ErrExpired and NO transaction is created (no orphan transaction)",
            "E19", "AUTO-API", "curl POST /v1/money-requests/{id}/pay concurrently with a sweeper trigger via a test script, count the transactions created"),
        row("TC-PER-020", f, R, "Negative", "P1",
            "User A creates a bill-split group for 3 people",
            "1. Create a bill-split request for 3 people with a fixed total amount\n2. 1 person pays, 2 have not paid\n3. Open the group progress tracking screen\n4. Press \"remind\" for the people still owing",
            "Total bill 900,000 VND / 3 people = 300,000 VND/person",
            "The tracking screen shows the avatar of the person who paid (bright) and those who have not (dimmed), the progress bar shows exactly 300,000/900,000 (33%), the \"remind\" button resends a notification to those who have not paid",
            "", "PARTIAL", "curl POST /v1/money-requests (split group) + check progress via API — the progress bar UI needs a human to confirm"),
    ]

    f = "Receive money via QR"
    rows += [
        row("TC-PER-021", f, R, "Happy Path", "P0",
            "User A (wallet owner) and User B (payer) both have active wallets",
            "1. User A opens the My QR screen without entering an amount\n2. User B scans the code with the app camera\n3. App B pre-fills the recipient info, User B enters the amount and confirms with PIN",
            "QR without a preset amount, User B enters 150,000 VND",
            "App B pre-fills the correct recipient info (User A) from the QR code; after PIN confirmation, the money is transferred successfully to User A",
            "", "HUMAN", "Needs a real camera to scan the QR — cannot be simulated via API/browser tool"),
        row("TC-PER-022", f, R, "Negative", "P1",
            "User A creates a QR with a specific preset amount",
            "1. User A creates a QR with a fixed amount of 50,000 VND\n2. User B scans the code\n3. Check whether the amount field on User B's screen can be edited",
            "QR with preset 50,000 VND",
            "User B's transfer form is pre-filled with exactly 50,000 VND and does not allow editing the amount — only PIN confirmation is needed to transfer",
            "", "HUMAN", "Needs a real camera to scan a QR with a preset amount"),
        row("TC-PER-023", f, R, "Negative", "P1",
            "There is a QR code that is expired or has a tampered payload",
            "1. Scan an invalid QR code (digital signature expired or payload modified)\n2. Observe the app's response",
            "QR with expired digital signature / modified payload",
            "The app clearly rejects with \"QR code is invalid or expired\", does not pre-fill the transfer form, does not allow continuing",
            "", "HUMAN", "Needs a real camera to scan the invalid test QR code"),
    ]

    f = "Savings vaults & auto round-up"
    rows += [
        row("TC-PER-024", f, R, "Happy Path", "P1",
            "User is logged in with an active wallet",
            "1. Create a new savings vault, set a name + amount goal\n2. Deposit money into the vault\n3. Withdraw some money from the vault\n4. Check the ledger of both operations",
            "Goal 5,000,000 VND, deposit 200,000 VND, withdraw 50,000 VND",
            "Each deposit/withdraw creates its own ledger-recorded transaction, the vault balance and the main wallet balance update correctly and match exactly",
            "", "PARTIAL", "curl POST /v1/vaults + /v1/vaults/{id}/deposit + /withdraw, confirm the ledger via API"),
        row("TC-PER-025", f, R, "Negative", "P2",
            "User has a savings vault with round-up mode enabled",
            "1. Enable \"round-up\" for a vault\n2. Make a P2P transfer with an odd amount (not a round thousand)\n3. Check whether the vault received the correct round-up remainder",
            "Transfer 123,700 VND → rounded up to 124,000, remainder 300 VND goes into the vault",
            "After the P2P transaction, the savings vault is automatically credited exactly the 300 VND round-up remainder, with no deviation",
            "", "PARTIAL", "curl POST transfer with an odd amount + GET /v1/vaults/{id} to check the vault balance after the transaction"),
        row("TC-PER-026", f, R, "Negative", "P2",
            "User has a savings vault with a certain balance",
            "1. Try to withdraw more than the current balance in the vault\n2. Observe the response",
            "Vault balance 100,000 VND, try to withdraw 500,000 VND",
            "The request is clearly rejected for insufficient vault balance, no transaction is created, the vault balance is unchanged",
            "", "AUTO-API", "curl POST /v1/vaults/{id}/withdraw exceeding the balance, confirm a 400/422 response"),
    ]

    f = "Transaction history & timeline"
    rows += [
        row("TC-PER-027", f, R, "Happy Path", "P1",
            "User already has at least a few transactions in history",
            "1. Open the history list, filter by time/transaction type\n2. Tap a transaction to view its detail\n3. View the timeline of each processing step",
            "User has at least 3 transactions of different types (topup, transfer, request)",
            "The list filters correctly by the criteria; the detail screen shows the full timeline of each processing step (not just success/failure) with the time of each step",
            "", "PARTIAL", "curl GET /v1/transactions?filter=... + GET /v1/transactions/{id}/timeline, cross-check the data — the UI needs a human to confirm the display"),
        row("TC-PER-028", f, R, "Negative", "P2",
            "User is logged in, on the home screen",
            "1. Open the app to the home screen, observe the balance display\n2. Tap the balance area to reveal it temporarily",
            "No special data needed",
            "The wallet balance is hidden by default (masked with ***/dots) when the app opens; after tapping, the balance appears temporarily then hides again after leaving the screen or on timeout",
            "", "HUMAN", "Pure UI behavior (hide/show balance) — needs a person to observe directly on the real app"),
    ]

    assert len(rows) == 28, f"sheet01 expected 28 rows, got {len(rows)}"
    return rows


# ---------------------------------------------------------------------------
# Sheet 02 — 02-business-maker-checker.xlsx (26 cases, GROUP=BIZ)
# ---------------------------------------------------------------------------
def sheet02_rows():
    MAKER = "Business Maker"
    CHECKER = "Business Checker"
    OADMIN = "Org Admin"
    rows = []

    f = "Create organization & invite members"
    rows += [
        row("TC-BIZ-001", f, OADMIN, "Happy Path", "P0",
            "User already has a personal account and wants to create a business wallet",
            "1. Create an organization, set a name\n2. Invite members by phone number\n3. Assign roles: maker / checker (with level)\n4. Check that the members appear with the correct permissions",
            "Organization name, phone numbers of 2 members (1 maker, 1 checker L1)",
            "The organization is created, the invited members appear in the list with the assigned role and checker level",
            "", "PARTIAL", "curl POST /v1/orgs + /v1/orgs/{id}/members, confirm the role via API — the invite-member UI needs a human to confirm"),
        row("TC-BIZ-002", f, OADMIN, "Negative", "P1",
            "On the organization invite-member screen",
            "1. Enter a phone number that has never registered with HPay\n2. Send the invitation\n3. Observe the invitation status",
            "Phone number that does not exist in the system",
            "The system handles it clearly: the invitation stays pending until that phone number registers, no crash, no fake successful invitation",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/members with an unregistered phone number, confirm the response/pending state"),
        row("TC-BIZ-003", f, OADMIN, "Negative", "P2",
            "Organization has no approval policy set up yet",
            "1. Assign a member the checker level 2 role\n2. Check the warning when there is no policy for that level yet",
            "Checker level 2, the organization has no tier policy using L2",
            "The system clearly warns the Org Admin that checker L2 has no applicable policy, avoiding the misunderstanding that the checker is ready to operate",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/policies and compare with the assigned level — the warning UI needs a human to confirm"),
    ]

    f = "Approval policy setup"
    rows += [
        row("TC-BIZ-004", f, OADMIN, "Happy Path", "P0",
            "Organization has at least 1 checker L1 and 1 checker L2",
            "1. Set the policy name and the applicable amount range\n2. Define tier 1 (under 5M → 1 L1 signature) and tier 2 (over 5M → L1+L2)\n3. Save the policy",
            "Tier 1: <5,000,000 VND / 1 L1 signature; Tier 2: ≥5,000,000 VND / L1+L2",
            "The policy is saved successfully with a new version, applied immediately to disbursement orders created after the save time",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/policies, confirm the tier structure via API"),
        row("TC-BIZ-005", f, OADMIN, "Negative", "P1",
            "Organization has a disbursement order awaiting approval under the current policy",
            "1. While the order is awaiting approval, the Org Admin edits the policy (changes the tier threshold)\n2. Check whether the pending order's signature requirement changes",
            "1 pending disbursement order, old policy version vs new version",
            "The order awaiting approval continues under the old policy version (keeps its original signature requirement); only newly created orders apply the edited policy",
            "E09", "PARTIAL", "curl PUT /v1/orgs/{id}/policies then GET the old order, confirm policy_snapshot is unchanged"),
        row("TC-BIZ-006", f, OADMIN, "Edge Case", "P0",
            "Maker is about to create a disbursement order exactly while the Org Admin is editing the policy",
            "1. Trigger concurrently: create a disbursement order (reads the current policy) and edit the policy (writes a new version)\n2. Check the policy_snapshot attached to the newly created order",
            "1 disbursement order created concurrently with 1 policy update",
            "The order's policy_snapshot captures exactly 1 policy as of the creation time — not mixed between old/new versions due to a race condition",
            "E09", "AUTO-API", "curl POST to create a disbursement order in parallel with a PUT policy via a test script, check the order's policy_snapshot"),
    ]

    f = "Create disbursement order"
    rows += [
        row("TC-BIZ-007", f, MAKER, "Happy Path", "P0",
            "Maker belongs to an organization that already has an approval policy",
            "1. Choose the recipient + amount + disbursement reason\n2. The system automatically looks up the matching policy\n3. Check the status of the newly created order",
            "Amount 3,000,000 VND (falls in tier 1)",
            "The order is created in the WAIT_APPROVAL state, the correct tier-1 signature requirement is applied, and the relevant checkers receive a notification",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/disbursements, confirm policy_snapshot and status via API"),
        row("TC-BIZ-008", f, MAKER, "Negative", "P0",
            "Maker belongs to an organization that already has an approval policy",
            "1. Try to create a disbursement order with an amount of 0 or a negative number\n2. Observe the response",
            "Amounts 0 and -100,000",
            "Both requests are rejected at the validation layer, no disbursement order is created",
            "", "AUTO-API", "curl POST /v1/orgs/{id}/disbursements with amount=0 and a negative amount, confirm a 400 response"),
        row("TC-BIZ-009", f, MAKER, "Negative", "P1",
            "Maker belongs to an organization that already has an approval policy",
            "1. Enter a recipient that does not exist in the system (invalid phone number)\n2. Confirm order creation",
            "Recipient phone number does not exist",
            "The request is clearly rejected, no pending disbursement order is created for an unidentifiable recipient",
            "", "AUTO-API", "curl POST /v1/orgs/{id}/disbursements with an invalid recipient, confirm an error response"),
    ]

    f = "Track disbursement order status"
    rows += [
        row("TC-BIZ-010", f, MAKER, "Happy Path", "P1",
            "Maker has a tier-2 disbursement order (needs L1+L2) being signed in sequence",
            "1. Open the \"Order status\" screen right after creation\n2. Follow each milestone: order created → L1 signs → L2 signs → money transferred",
            "Tier-2 order, 2 checkers sign in turn",
            "Each milestone shows the exact time and the name of the signer; once enough signatures are collected, the money is transferred automatically with no further action from the maker",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id}/timeline after each signature, cross-check the timestamps"),
        row("TC-BIZ-011", f, MAKER, "Negative", "P1",
            "Maker has a disbursement order rejected by 1 checker",
            "1. The checker rejects the order\n2. The maker opens the order status screen",
            "1 checker presses Reject with a reason",
            "The status screen clearly shows the order was rejected, with the checker's name and the rejection reason",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id} after the reject, confirm status + reason in the response"),
    ]

    f = "Cancel a pending order"
    rows += [
        row("TC-BIZ-012", f, MAKER, "Happy Path", "P1",
            "Maker has a disbursement order that nobody has signed yet",
            "1. Go to the status screen of the pending order\n2. Press Cancel order and confirm again\n3. Check the status",
            "Order with no signatures",
            "The order moves to the \"cancelled\" state, no checker can sign it anymore",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/disbursements/{id}/cancel, confirm status=cancelled"),
        row("TC-BIZ-013", f, MAKER, "Negative", "P0",
            "Maker has a disbursement order that is fully signed and the money has already been transferred",
            "1. Try to press Cancel order on a completed order\n2. Observe the response",
            "Order in the completed state",
            "The cancel request is clearly rejected with a message that the order is completed and cannot be cancelled — no money is reversed unintentionally",
            "", "AUTO-API", "curl POST cancel on a completed order, confirm a 400/409 response"),
        row("TC-BIZ-014", f, MAKER, "Negative", "P1",
            "Maker has a disbursement order that has expired (expired due to E10)",
            "1. Try to press Cancel order on an expired order\n2. Observe the response",
            "Order in the expired state",
            "The cancel request is rejected because the order is already in a terminal state (expired), and does not change state again",
            "", "AUTO-API", "curl POST cancel on an expired order, confirm a clear error response"),
    ]

    f = "Approval inbox (My Tasks — Checker)"
    rows += [
        row("TC-BIZ-015", f, CHECKER, "Happy Path", "P0",
            "Checker L1 has at least 1 order of the matching level awaiting signature",
            "1. Open the \"Awaiting my approval\" inbox\n2. Check that the list shows only orders of the right level that are still valid",
            "Checker L1, 1 pending tier-1 order",
            "The inbox shows exactly the orders that this checker L1 needs to sign, and does not show orders of other levels or expired orders",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/my-tasks with the checker L1 token, cross-check the returned list"),
        row("TC-BIZ-016", f, CHECKER, "Negative", "P1",
            "There is a tier-2 order that needs both L1 and L2 signatures, L1 has not signed",
            "1. Log in with a checker L2 account\n2. Open the \"Awaiting my approval\" inbox",
            "Tier-2 order, L1 has not signed",
            "The order does NOT appear in checker L2's inbox until L1 has finished signing (correct turn order)",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/my-tasks with the checker L2 token before L1 signs, confirm the order is not in the list"),
        row("TC-BIZ-017", f, CHECKER, "Negative", "P1",
            "There is an order that just moved to the expired state (E10)",
            "1. Open the \"Awaiting my approval\" inbox of the relevant checker after the order expired",
            "Order just expired",
            "The order no longer appears in the checker's approval inbox after it has expired",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/my-tasks after the order expired, confirm it is no longer in the list"),
    ]

    f = "View order detail + signature history"
    rows += [
        row("TC-BIZ-018", f, CHECKER, "Happy Path", "P1",
            "There is a tier-2 order already signed by checker L1, awaiting L2",
            "1. Checker L2 opens the order from the inbox\n2. View the amount, creator (maker), disbursement reason\n3. View the \"Signatures\" section: who signed at which level, at what time",
            "Tier-2 order with 1 L1 signature",
            "The detail screen shows everything accurately: amount, maker, reason, and the list of existing signatures (checker L1 name + signing time)",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id} + /approvals, cross-check the returned data"),
        row("TC-BIZ-019", f, CHECKER, "Negative", "P2",
            "There is a newly created order that nobody has signed yet",
            "1. The checker opens the order from the inbox right after the maker creates it",
            "Newly created order, 0 signatures",
            "The \"Signatures\" section is clearly empty (not an error or a misleading blank screen), the checker understands they are the first signer",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id}/approvals right after creation, confirm an empty array"),
    ]

    f = "Approve/reject with OTP"
    rows += [
        row("TC-BIZ-020", f, CHECKER, "Happy Path", "P0",
            "Checker L1 has a tier-1 order (needs only 1 signature) awaiting signature",
            "1. Press \"Approve\"\n2. The system sends an OTP code to the checker's phone\n3. Enter the correct OTP to confirm the signature",
            "Valid mock OTP",
            "The signature is recorded as the last required signature → money is transferred immediately, the order moves to completed",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/disbursements/{id}/approve with a valid OTP, confirm status + balance"),
        row("TC-BIZ-021", f, CHECKER, "Negative", "P0",
            "Checker is on the OTP signature confirmation step",
            "1. Press \"Approve\"\n2. Enter a wrong OTP\n3. Check whether the order counts as signed",
            "Wrong OTP",
            "The signature is NOT recorded, the order stays in the pending state exactly as before pressing Approve",
            "", "AUTO-API", "curl POST approve with a wrong OTP, confirm a 401/422 response and that the order state is unchanged"),
        row("TC-BIZ-022", f, CHECKER, "Edge Case", "P0",
            "Tier-1 order needs only 1 signature, 2 checkers of the same level both have signing rights",
            "1. Checker A and checker B press \"Approve\" almost simultaneously (valid OTPs already available)\n2. Check the outcome of both requests",
            "2 approve requests sent in parallel, same tier-1 order",
            "FOR UPDATE + UNIQUE(approval_id, checker_id) guarantee exactly 1 checker wins and the order is finalized exactly once; the other checker receives a clear response that the order has already been processed",
            "E08", "AUTO-API", "curl POST approve in parallel from 2 checkers (test script sends them concurrently), count the actual finalizations"),
        row("TC-BIZ-023", f, CHECKER, "Edge Case", "P0",
            "There is an organization disbursement order awaiting approval for more than 24 hours",
            "1. Set the order creation time back beyond 24h (simulated)\n2. Wait for the expiry cron to run (or trigger it manually)\n3. Check the order and workflow state",
            "Order pending for more than 24h",
            "expires_at + the sweeping cron move the approval to expired, the workflow instance fails cleanly, no funds are left dangling",
            "E10", "AUTO-API", "curl UPDATE created_at into the past via a test tool then trigger the cron/expiry endpoint, confirm status=expired"),
    ]

    f = "Result after signing"
    rows += [
        row("TC-BIZ-024", f, CHECKER, "Happy Path", "P0",
            "Checker has just placed the last signature required for an order",
            "1. After signing, view the result on screen immediately",
            "Tier-1 order, the single signature has just completed it",
            "The screen clearly shows \"Approved — transfer in progress\", unambiguously",
            "", "PARTIAL", "curl POST approve, compare the response status with the expected completed/processing"),
        row("TC-BIZ-025", f, CHECKER, "Negative", "P1",
            "Checker L1 has just signed a tier-2 order (L2 still needs to sign)",
            "1. Finish signing as L1\n2. View the result on screen",
            "Tier-2 order, only 1/2 signatures so far",
            "The screen clearly shows \"recorded, waiting for 1 more person\", and does not wrongly report completion",
            "", "PARTIAL", "curl POST approve as L1, cross-check how many signatures the response says are still missing"),
        row("TC-BIZ-026", f, CHECKER, "Edge Case", "P0",
            "Tier-1 order needs only 1 signature, checker B presses sign a few seconds after checker A",
            "1. Checker A signs first and wins the race\n2. Checker B signs a few seconds later on the same order",
            "2 checkers sign a few seconds apart on the same tier-1 order",
            "Checker B gets a screen clearly saying \"this order was just processed by someone else\" instead of an incomprehensible system error (not a generic HTTP 500)",
            "E08", "AUTO-API", "curl POST approve by checker A then checker B a few seconds apart, confirm B's response is a 409 + clear message"),
    ]

    assert len(rows) == 26, f"sheet02 expected 26 rows, got {len(rows)}"
    return rows


# ---------------------------------------------------------------------------
# Sheet 03 — 03-admin-ops.xlsx (21 cases, GROUP=ADM) — all AUTO-WEB
# ---------------------------------------------------------------------------
def sheet03_rows():
    R = "Ops Admin"
    METHOD_BASE = "Browser tool: preview_start hpay-admin dev server, navigate + computer + read_page/get_page_text, screenshot as Evidence"
    rows = []

    f = "Overview dashboard"
    rows += [
        row("TC-ADM-001", f, R, "Happy Path", "P1",
            "Admin is logged in to hpay-admin, demo data is present (seed-demo-data)",
            "1. Log in, open the Dashboard page\n2. View the daily KPI metrics (transaction count, KYC pending review, fraud alerts)",
            "Seeded admin account, demo data already loaded",
            "The Dashboard shows the KPI metrics correctly, matching the real data in the system (cross-checked via API)",
            "", "AUTO-WEB", METHOD_BASE + " on /dashboard"),
        row("TC-ADM-002", f, R, "Negative", "P2",
            "There is at least 1 \"needs attention now\" item (e.g. KYC waiting long, unhandled fraud)",
            "1. Open the Dashboard\n2. Click the \"needs attention now\" area\n3. Check the navigation",
            "1 overdue pending KYC profile seeded in advance",
            "Clicking the needs-attention area navigates to the correct related queue (e.g. /kyc), not to a wrong page",
            "", "AUTO-WEB", METHOD_BASE + " — click the alert area, confirm the destination URL"),
    ]

    f = "KYC profile review queue"
    rows += [
        row("TC-ADM-003", f, R, "Happy Path", "P0",
            "There is at least 1 KYC profile in the queue (AI score not high enough to auto-approve) — user 0900000003 per the seed",
            "1. Open /kyc\n2. Select a profile, view the ID card + selfie photos + the AI score and reason\n3. Press Approve",
            "Seed user 0900000003, ai_score 64",
            "The profile moves to approved, the user's daily transaction limit is raised according to the new KYC tier",
            "", "AUTO-WEB", METHOD_BASE + " on /kyc/{id}, click Approve, read the toast + call the API to confirm status"),
        row("TC-ADM-004", f, R, "Negative", "P1",
            "There is 1 KYC profile in the queue",
            "1. Open the profile detail\n2. Press Reject, enter a reason",
            "1 pending profile",
            "The profile moves to rejected, the user receives a notification and can submit a new profile",
            "", "AUTO-WEB", METHOD_BASE + " on /kyc/{id}, click Reject, confirm the state"),
        row("TC-ADM-005", f, R, "Negative", "P2",
            "There are ≥2 KYC profiles in the queue",
            "1. Use a keyboard shortcut to quickly approve the selected profile\n2. Check that the next profile is focused automatically",
            "≥2 pending profiles",
            "The shortcut approves the selected profile, and the cursor automatically moves to the next profile in the queue",
            "", "AUTO-WEB", METHOD_BASE + " — use computer key to send the shortcut, read the DOM to confirm focus"),
    ]

    f = "Fraud monitoring"
    rows += [
        row("TC-ADM-006", f, R, "Happy Path", "P0",
            "There is ≥1 transaction flagged as suspected fraud (fraud flag) in the seed",
            "1. Open /fraud\n2. View the reason for the flag (hard rules + AI explanation)\n3. Press \"Confirm fraud\"",
            "1 pre-seeded fraud flag (user 0900000004)",
            "The transaction/flag moves to confirmed_fraud, the related account is locked or put into the reversal process",
            "", "AUTO-WEB", METHOD_BASE + " on /fraud/{id}, click Confirm, confirm the state via API"),
        row("TC-ADM-007", f, R, "Negative", "P1",
            "There is 1 fraud flag in the queue",
            "1. Open the flag detail\n2. Press \"Dismiss alert\" (false positive)",
            "1 pending fraud flag",
            "The flag moves to dismissed, the related transaction/account is not locked",
            "", "AUTO-WEB", METHOD_BASE + " on /fraud/{id}, click Dismiss, confirm the state"),
        row("TC-ADM-008", f, R, "Negative", "P2",
            "There is 1 fraud flag past its handling deadline",
            "1. Open the /fraud list, filter by \"overdue\"\n2. Check the visual warning display for the overdue flag",
            "1 fraud flag whose deadline has passed",
            "The overdue flag is visually highlighted (color/badge), distinct from flags still within the deadline",
            "", "AUTO-WEB", METHOD_BASE + " on /fraud with the overdue filter, read the DOM to confirm the badge"),
    ]

    f = "Workflow Monitor"
    rows += [
        row("TC-ADM-009", f, R, "Happy Path", "P1",
            "There are many workflow instances in different states (running/failed/stuck)",
            "1. Open /workflows\n2. Filter by status/transaction type\n3. Click an instance to view its detail",
            "Diverse workflow data from seed-demo-data",
            "The list filters correctly by the criteria; the detail screen shows the step where it is stopped and the reason",
            "", "AUTO-WEB", METHOD_BASE + " on /workflows, apply a filter, click 1 row, read the step detail"),
        row("TC-ADM-010", f, R, "Edge Case", "P1",
            "There is a workflow instance stuck because a worker died midway (DEBIT done, CREDIT not yet — E04/E05)",
            "1. Open /workflows/{id} of the stuck instance\n2. Check that the stopped step is displayed correctly, with no double side effect",
            "1 stuck instance simulating E04/E05",
            "The instance detail shows the correct pending step; after the system recovers, there is no repeated DEBIT/CREDIT (cross-checked against the ledger via API)",
            "E04", "AUTO-WEB", METHOD_BASE + " on /workflows/{id} + curl GET ledger to confirm no double side effect"),
        row("TC-ADM-011", f, R, "Edge Case", "P1",
            "There is a workflow instance stuck for too long (updated_at pushed back past the threshold)",
            "1. Trigger the sweeper (recovery) via an endpoint/CI script\n2. Open /workflows/{id} and check the state updated after the sweeper ran",
            "1 stuck instance, sweeper runs 2 times in parallel (simulating E20)",
            "The instance is claimed and re-emitted exactly once even though 2 sweepers run in parallel; the Workflow Monitor screen reflects the correct final state, with no repeated steps",
            "E20", "AUTO-WEB", METHOD_BASE + " on /workflows/{id} after triggering parallel sweepers via a test script"),
    ]

    f = "Daily transaction reconciliation"
    rows += [
        row("TC-ADM-012", f, R, "Happy Path", "P1",
            "Transaction data + mock bank statement have been seeded for 1 day",
            "1. Open /recon, press \"Run reconciliation\"\n2. View the matched/mismatched/missing list\n3. Press Export Excel",
            "1 day of demo reconciliation data",
            "The reconciliation result shows the 3 groups correctly (matched/mismatched/missing); the exported Excel file downloads successfully and matches the data on screen",
            "", "AUTO-WEB", METHOD_BASE + " on /recon, click Run + Export Excel, confirm the download link appears"),
        row("TC-ADM-013", f, R, "Negative", "P2",
            "There is a reconciliation result with at least 1 mismatched item",
            "1. Open a mismatched item in the reconciliation result\n2. Read the AI-written summary of the cause of the mismatch",
            "1 mismatched item in the recon result",
            "The AI summary is shown in easy-to-understand natural language and points in the right direction on the cause of the mismatch (missing on which side)",
            "", "AUTO-WEB", METHOD_BASE + " on /recon, open the mismatched item's detail, read the AI summary content"),
        row("TC-ADM-014", f, R, "Edge Case", "P1",
            "There is a transaction occurring right across the midnight (00:00) boundary of the reconciliation day",
            "1. Run reconciliation for a day with a transaction straddling midnight\n2. Check that this transaction is classified correctly and no false mismatch is reported",
            "1 transaction with a timestamp near 00:00",
            "Recon matches by ref_code, not only by calendar day — the cutoff-crossing transaction is classified separately and clearly, and does not fall into the \"mismatch\" group due to clock skew",
            "E16", "AUTO-WEB", METHOD_BASE + " on /recon with midnight-cutoff transaction data, check the result classification"),
    ]

    f = "Admin My Tasks"
    rows += [
        row("TC-ADM-015", f, R, "Happy Path", "P0",
            "Admin A has proposed a sensitive action (refund) awaiting Admin B's approval",
            "1. Log in as Admin B, open /tasks\n2. View Admin A's proposal reason\n3. Press Approve",
            "1 pending reversal request from Admin A",
            "The action (refund) is actually executed after Admin B approves; My Tasks updates its state in place immediately",
            "", "AUTO-WEB", METHOD_BASE + " on /tasks, click Approve, confirm via the ledger/reversal API"),
        row("TC-ADM-016", f, R, "Negative", "P1",
            "Admin A has proposed a sensitive action awaiting Admin B's approval",
            "1. Log in as Admin B\n2. Press Reject with a reason",
            "1 pending task",
            "The action stops and is not executed; the task moves to the rejected state with the reason",
            "", "AUTO-WEB", METHOD_BASE + " on /tasks, click Reject, confirm the state"),
        row("TC-ADM-017", f, R, "Edge Case", "P0",
            "There is a task in My Tasks that 2 admins (2 browser contexts) both have the right to handle",
            "1. Open 2 browser contexts (2 different admin login sessions) looking at the same task\n2. Both press \"Approve\" almost simultaneously",
            "1 task, 2 contexts acting in parallel",
            "1 context receives 200 + the row updates correctly; the other context receives a 409 ERR_ALREADY_FINALIZED error with a clear toast, the row updates in place, without reloading the whole table",
            "A01", "AUTO-WEB", "Browser tool: 2 separate tabs (2 tabs_create) logged in as 2 admins, click Approve almost simultaneously, read the network response + DOM"),
    ]

    f = "Audit log search"
    rows += [
        row("TC-ADM-018", f, R, "Happy Path", "P1",
            "There are many audit_logs records from earlier actions (KYC approve, policy change...)",
            "1. Open /audit\n2. Type a search keyword (e.g. an admin name or action type)\n3. View the results",
            "Keyword matching ≥1 existing audit record",
            "The search results show correctly: who, did what, when, why — matching the real audit_logs data",
            "", "AUTO-WEB", METHOD_BASE + " on /audit, enter a search keyword, cross-check the results with the API"),
        row("TC-ADM-019", f, R, "Negative", "P2",
            "There is ≥1 audit log record",
            "1. Try to find a way to edit/delete an audit log record (there is no action on the UI for this)\n2. Confirm via API that there is also no update/delete endpoint",
            "1 specific audit log record",
            "No edit/delete action is available on the UI; calling the update/delete API directly (if attempted) is rejected — the audit log is immutable",
            "", "AUTO-WEB", METHOD_BASE + " on /audit — confirm the UI has no edit/delete button + curl try PUT/DELETE on the audit endpoint"),
    ]

    f = "Organization & policy management"
    rows += [
        row("TC-ADM-020", f, R, "Happy Path", "P1",
            "There is ≥1 active business organization in the system (seed)",
            "1. Open /orgs\n2. Select an organization\n3. View the members + roles + the current disbursement approval policy",
            "1 seeded organization with members + policy",
            "The detail page shows the member list, each person's role, and the current disbursement approval policy correctly",
            "", "AUTO-WEB", METHOD_BASE + " on /orgs/{id}, cross-check the displayed data with the API"),
        row("TC-ADM-021", f, R, "Negative", "P2",
            "Admin is viewing an organization's detail and needs to adjust the policy on the customer's behalf",
            "1. Open the organization's policy edit screen\n2. Adjust a tier, save\n3. Check that the change takes effect",
            "1 organization, edit the tier threshold",
            "The policy is updated to a new version, displayed correctly on the UI and via API, without affecting pending orders (per E09)",
            "", "AUTO-WEB", METHOD_BASE + " on /orgs/{id}, edit the policy, confirm the new version via API"),
    ]

    assert len(rows) == 21, f"sheet03 expected 21 rows, got {len(rows)}"
    return rows


# ---------------------------------------------------------------------------
# Sheet 04 — 04-platform-crosscutting.xlsx (10 cases, GROUP=PLT) — all AUTO-API
# ---------------------------------------------------------------------------
def sheet04_rows():
    R = "System"
    rows = []

    f = "Automatic fraud detection"
    rows += [
        row("TC-PLT-001", f, R, "Happy Path", "P1",
            "hpay-backend is running, a user with an active wallet exists",
            "1. Make 1 normal transaction (reasonable amount, familiar device)\n2. Check whether the transaction is blocked",
            "Transaction of 100,000 VND, normal device/frequency",
            "Hard rules score a low risk → the transaction passes immediately, with no AI intervention and no abnormal delay",
            "", "AUTO-API", "curl POST /v1/transactions/transfer, measure latency + confirm no fraud flag is raised"),
        row("TC-PLT-002", f, R, "Negative", "P1",
            "hpay-backend is running",
            "1. Create a transaction with characteristics falling in the \"gray zone\" (e.g. slightly high frequency but not certain)\n2. Check whether the system calls the AI for an additional explanation",
            "Transaction designed to fall exactly at the rule engine's gray-zone threshold",
            "The system calls the AI to further explain the reason for suspicion; the AI does NOT block the money on its own — the transaction is still recorded, pending the hard rules/a human for the final decision",
            "", "AUTO-API", "curl POST transfer with gray-zone characteristics, check fraud_flags has a record with ai_explanation and the transaction is not blocked"),
        row("TC-PLT-003", f, R, "Negative", "P0",
            "hpay-backend is running",
            "1. Create a transaction with clearly dangerous characteristics (far beyond the hard-rule threshold)\n2. Check whether the transaction is blocked immediately",
            "Transaction far exceeding the rule engine's abnormal velocity/amount threshold",
            "The hard rules block it immediately without waiting for the AI; the block decision comes from the hard rules, not from an AI decision",
            "", "AUTO-API", "curl POST transfer with clearly dangerous characteristics, confirm the response is blocked (403/422) immediately"),
    ]

    f = "Retry safety (Idempotency)"
    rows += [
        row("TC-PLT-004", f, R, "Happy Path", "P0",
            "hpay-backend is running, the user has an active wallet",
            "1. Send POST /v1/transactions/transfer with Idempotency-Key X (1st time)\n2. Resend the exact same request with the same Idempotency-Key X (2nd time, simulating a re-press)\n3. Compare the 2 responses",
            "Idempotency-Key = X (fixed UUID), same payload",
            "Both responses return the same refCode/transaction ID; the wallet is debited/credited exactly once",
            "E01", "AUTO-API", "curl POST transfer twice with the same Idempotency-Key header, compare response + balance before/after"),
        row("TC-PLT-005", f, R, "Edge Case", "P0",
            "A workflow instance has already published a result event once",
            "1. Simulate the outbox relay publishing exactly the same event a 2nd time (retry)\n2. Check whether side effects (e.g. crediting money, sending a notification) are repeated",
            "1 result event, published 2 times in a row",
            "Consumer checks step_executions (instance, step, attempt) before side effects — the 2nd event is skipped, no duplicate side effect",
            "E06", "AUTO-API", "curl/script to re-trigger the exact event via the outbox relay test hook, count the actual side effects (e.g. number of ledger rows)"),
        row("TC-PLT-006", f, R, "Negative", "P1",
            "hpay-backend is running, the topup and transfer endpoints both accept an Idempotency-Key",
            "1. Send a topup request with key Y, repeated 2 times\n2. Send a transfer request with key Z (different from Y), repeated 2 times\n3. Check that both flows dedupe correctly by their own key",
            "Idempotency-Key Y (topup) and Z (transfer) independent of each other",
            "Each flow creates exactly 1 transaction record for its own key, with no mixing between the 2 request types sharing the same idempotency mechanism",
            "E01", "AUTO-API", "curl POST topup x2 (key Y) + POST transfer x2 (key Z), confirm each type creates exactly 1 transaction"),
    ]

    f = "Self-recovery on failure"
    rows += [
        row("TC-PLT-007", f, R, "Happy Path", "P0",
            "A transaction stopped midway due to a simulated failure (network loss/server restart)",
            "1. Create a transaction then make it \"stuck\" midway (simulating a dead worker)\n2. Wait for the periodic sweeping process (recovery sweeper) to run\n3. Check whether the transaction is processed further",
            "1 artificially stuck workflow instance",
            "The transaction is retried exactly from the interrupted step (without repeating completed steps), and finally completes or fails clearly with a refund if needed",
            "", "AUTO-API", "curl trigger the sweeper endpoint/script + GET the workflow instance state before/after"),
        row("TC-PLT-008", f, R, "Edge Case", "P0",
            "Redis is running normally and can be temporarily stopped in the test environment",
            "1. Stop Redis completely\n2. Send a request to create a new transaction\n3. Check the response and the DB state",
            "Redis down, 1 request to create a new transaction",
            "The API still returns a successful response (202), the transaction is written to the DB + outbox; the engine pauses further processing (waiting for Redis) but NO data is lost, no 500 error",
            "E17", "AUTO-API", "docker stop the redis container then curl POST transfer, check the response + DB records, then restart Redis and confirm the engine resumes"),
        row("TC-PLT-009", f, R, "Edge Case", "P1",
            "A transaction just committed to the DB successfully but event publish failed (simulated broker outage)",
            "1. Simulate a broker (Redis Streams) outage right after the DB commit\n2. Restore the broker\n3. Check whether the outbox relay republishes the event",
            "1 transaction, short broker outage right after the commit",
            "The event sits in the outbox table (transactional outbox) while the broker is down; after the broker recovers, the outbox relay republishes automatically, no event is lost",
            "E05", "AUTO-API", "docker stop redis temporarily right when the transaction commits, restart it, check outbox.published_at changes from NULL to a value"),
        row("TC-PLT-010", f, R, "Edge Case", "P1",
            "A workflow instance is marked \"stuck\" (updated_at pushed back past the threshold)",
            "1. Run 2 sweeper processes concurrently on the same stuck instance\n2. Count the successful claims and the number of event re-emits",
            "1 stuck instance, 2 sweepers running in parallel (goroutine test)",
            "Lease claim via UPDATE ... RETURNING guarantees only 1 sweeper claims successfully; the event is re-emitted exactly once, the instance is not recovered twice",
            "E20", "AUTO-API", "test script runs 2 goroutines calling SweepStuck() in parallel, query the outbox to count remaining unpublished events"),
    ]


    f = "Idempotency per user (BE-H1, ADR-0011)"
    rows += [
        row('TC-IDM-001', f, R, 'Happy Path', 'P0',
            'Funded sender and receiver, fresh key K',
            '1. Transfer 100,000 with key K\n2. Repeat the identical call with K',
            'Same user, same key K, same receiver/amount/note',
            'Second call returns the same ref_code flagged duplicate; exactly one transaction row; sender debited once',
            'E01', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_Transfer_ReplaySameUserSamePayload -race -count=1 -v'),
        row('TC-IDM-002', f, R, 'Negative', 'P0',
            'A transfer with key K already exists for user A',
            '1. Reuse K with a different amount\n2. Reuse K with a different receiver\n3. Reuse K with a different note',
            'Same user and key, one field changed each time',
            'Each call is rejected with ErrIdempotencyKeyReused (HTTP 409 ERR_IDEMPOTENCY_KEY_REUSED); still one transaction',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_Transfer_SameKeyDifferentPayloadRejected -race -count=1 -v'),
        row('TC-IDM-003', f, R, 'Security/AuthZ', 'P0',
            'Two users A and B',
            '1. A tops up 300,000 with key K\n2. B tops up 700,000 with the same K\n3. A repeats its call',
            'Same key K used by two users with different amounts',
            "B gets its own new transaction and balance 700,000 (no data from A); A's replay returns A's own transaction",
            'E01', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_TwoUsersSameKeyAreIndependent -race -count=1 -v'),
        row('TC-IDM-004', f, R, 'Negative', 'P0',
            'Funded user',
            '1. Call engine request, transfer, top-up, money-request create, split and pay with an empty key',
            'Empty Idempotency-Key',
            'Every call returns ErrIdempotencyKeyRequired (no server-generated key); nothing is created',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_MissingKeyRejectedInServices -race -count=1 -v'),
        row('TC-IDM-005', f, R, 'Edge Case', 'P0',
            'Funded sender with enough balance',
            '1. Fire 8 concurrent transfers with the same key and payload',
            '8 goroutines, same key K',
            'All return the same ref_code; one transaction row; sender debited exactly once',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_ConcurrentSameKeyOneTransaction -race -count=1 -v'),
        row('TC-IDM-006', f, R, 'Security/AuthZ', 'P0',
            'Users A and B, router with RequireIdempotencyKey',
            '1. POST /v1/transactions/request as A with K\n2. Same K, different amount, as A\n3. Same K as B',
            'service_code P2P_TRANSFER, amounts 100,000 / 200,000 / 300,000',
            "200; 409 ERR_IDEMPOTENCY_KEY_REUSED; 200 with B's own ref_code and amount",
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_EngineHTTP_ReplayConflictAndIsolation -race -count=1 -v'),
        row('TC-IDM-007', f, R, 'Happy Path', 'P1',
            'Requester, payer, and another requester',
            '1. Create a money request with K twice\n2. Create with K and a different amount\n3. Another requester creates with K',
            'payer_phone, amount 150,000, note',
            'Replay returns the same request id (one row); changed amount is ErrIdempotencyKeyReused; other requester gets an independent request',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_MoneyRequestCreate -race -count=1 -v'),
        row('TC-IDM-008', f, R, 'Edge Case', 'P1',
            'Requester and two payers',
            '1. Create a 2-payer split with K twice\n2. Repeat with a different total\n3. Repeat with different payers',
            'total 300,000, two payers',
            'Replay returns the original split (same split_group_id, ids and order, 2 rows); both variants are ErrIdempotencyKeyReused',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_MoneyRequestSplit -race -count=1 -v'),
        row('TC-IDM-009', f, R, 'Happy Path', 'P0',
            'Two pending money requests R1, R2 for one funded payer',
            '1. Pay R1 with K\n2. Pay R1 again with K\n3. Pay R2 with K',
            'Same payer and key K',
            'Replay returns the original transaction flagged duplicate; paying R2 with K is ErrIdempotencyKeyReused; one transaction',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_MoneyRequestPay -race -count=1 -v'),
        row('TC-IDM-010', f, R, 'Negative', 'P1',
            'User with an empty wallet',
            '1. Top up 100,000 with K\n2. Top up 200,000 with the same K',
            'Same user and key, different amount',
            'Second call is ErrIdempotencyKeyReused; balance stays 100,000',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_Topup_DifferentAmountRejected -race -count=1 -v'),
        row('TC-IDM-011', f, R, 'Boundary', 'P0',
            'Real router with auth, valid user token',
            '1. POST the four mounted money routes without the header and with it\n2. GET /v1/wallets without it\n3. POST /v1/money-requests/:id/decline without it',
            'Routes: transactions/request, money-requests, money-requests/split, org/:id/transactions',
            '400 ERR_IDEMPOTENCY_KEY_REQUIRED without the header; the middleware passes with it; reads and decline never require it',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestIdempotency_MountedMoneyRoutesRequireKey -race -count=1 -v'),
        row('TC-IDM-012', f, R, 'Edge Case', 'P0',
            'Scratch database migrated to 000014 with legacy rows (keyed and NULL-keyed)',
            '1. Apply 000015 up\n2. Check legacy request_hash\n3. Reuse a legacy key as another user and as the same user',
            'Existing transactions with idempotency keys',
            'Up succeeds; old request_hash is NULL; another user can reuse the key; same user cannot; NULL keys stay unconstrained',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestMigration000015_UpDownBackfill -race -count=1 -v'),
        row('TC-IDM-013', f, R, 'Edge Case', 'P0',
            'Database at 000015 where two users share a key',
            '1. Apply 000015 down\n2. Remove the clashing row\n3. Apply down again, then up again',
            'Two users, same key',
            'First down fails loudly; second succeeds and drops request_hash; up re-applies cleanly',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestMigration000015_UpDownBackfill -race -count=1 -v'),
        row('TC-IDM-014', f, R, 'Happy Path', 'P0',
            'User with wallet',
            '1. Top up twice with the same key and amount',
            'Regression of E01',
            'One transaction; credited once',
            'E01', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestEdge_E01_IdempotentTopup -race -count=1 -v'),
        row('TC-IDM-015', f, R, 'Negative', 'P1',
            'Engine route mounted with RequireIdempotencyKey',
            '1. POST /v1/transactions/request without the header',
            'No Idempotency-Key',
            'HTTP 400',
            '', "AUTO-API", 'Run via scripts/test-backend.ps1 (Docker): go test ./test/integration/... -run TestTransactionRequestHTTP_MissingIdempotencyKeyRejected -race -count=1 -v'),
    ]

    assert len(rows) == 25, f"sheet04 expected 25 rows, got {len(rows)}"
    return rows


# ---------------------------------------------------------------------------
# Sheet 05 — 05-edge-cases.xlsx (39 cases, GROUP=EDG) — 1 row per real edge ID
# ---------------------------------------------------------------------------
def sheet05_rows():
    rows = []
    for eid in EDGE_ORDER:
        desc, repo, test_name, expected = EDGE_CASES[eid]
        prefix = eid[0]
        if prefix == "E":
            role, executor = "System", "AUTO-API"
            method = f"curl/PowerShell directly against the corresponding hpay-backend API, cross-checked with {test_name} — compare response/DB state with the Expected Result"
            pre = f"hpay-backend running with a clean DB+Redis (or the simulated state the scenario needs); corresponds to the automated test {test_name} in src/backend/{'test/integration' if 'integration' in test_name.lower() or True else ''}"
        elif prefix == "M":
            role, executor = "Personal User / Business Checker", "HUMAN"
            method = "Needs a real device/Expo simulator — Claude has no tool to drive a mobile screen; the tester follows the scenario exactly and cross-checks with the corresponding JS unit test"
            pre = f"hpay-mobile running on a real device or simulator, valid demo account; corresponds to {test_name} in src/mobile/src/**/__tests__"
        else:  # A
            role, executor = "Ops Admin", "AUTO-WEB"
            method = f"Browser tool: preview_start hpay-admin dev server, 2 tabs/contexts when a race must be staged, computer/read_page/network to confirm behavior — corresponds to {test_name}"
            pre = "hpay-admin dev server running, ≥2 admin fixture accounts (maker/checker or 2 admins with the same rights) when the scenario needs 2 simultaneous actors"

        steps = (
            f"1. Set up the exact conditions of {eid}: {desc}\n"
            f"2. Perform the action that triggers the scenario (the corresponding transaction/API call/UI action)\n"
            f"3. Observe/measure the actual result (response, DB state, UI state) and compare with the Expected Result\n"
            f"4. Cross-check against the automated test {test_name} (if this case already has automated coverage) to confirm consistent behavior"
        )
        rows.append(row(
            f"TC-EDG-{eid}", f"Edge Case {eid} ({repo})", role, "Edge Case",
            "P0" if prefix in ("E", "M") else "P1",
            pre, steps, f"See the scenario description for {eid}: {desc}", expected,
            eid, executor, method,
        ))
    assert len(rows) == 39, f"sheet05 expected 39 rows, got {len(rows)}"
    ids = [r[0] for r in rows]
    assert len(set(ids)) == 39, "duplicate Test ID in sheet05"
    return rows


# ---------------------------------------------------------------------------
# Workbook builder
# ---------------------------------------------------------------------------
README_INTRO = (
    "A manual QA checklist that runs in parallel with the automated test suite (TestEdge_*) already present in "
    "hpay-backend / hpay-mobile / hpay-admin. It does not replace it and does not overlap its scope — "
    "see docs/qa/plan/implementation-plan.html and solution-architecture.html for the "
    "full context and design decisions."
)

SCHEMA_ROWS = [
    ("A", "Test ID", "TC-<GROUP>-<NNN> or TC-EDG-<E/M/A-ID>",
     "GROUP ∈ {PER, BIZ, ADM, PLT} for sheets 01-04 (numbered sequentially within the group); sheet 05 uses the original edge-case ID itself"),
    ("B", "Feature", "Text — feature name", "Sheets 01-04: exactly the name in FEATURES[].title of functional-overview.vi.html. Sheet 05: \"Edge Case <ID> (<repo>)\""),
    ("C", "Role / Actor", "Enum", "Personal User · Business Maker · Business Checker · Org Admin · Ops Admin · System"),
    ("D", "Test Type", "Enum", "Happy Path · Negative · Edge Case · Boundary · Security/AuthZ"),
    ("E", "Priority", "Enum", "P0 (release blocker, default for anything touching money) · P1 · P2"),
    ("F", "Precondition / Setup", "Multi-line text", "Seed data required before running the case, so the case is repeatable"),
    ("G", "Test Steps", "Text numbered 1. 2. 3.", "Detailed enough for a stranger (or Claude) to follow without asking again"),
    ("H", "Test Data", "Short text/JSON", "Demo phone numbers, specific amounts, mock OTP... kept separate from Steps"),
    ("I", "Expected Result", "Text", "Clearly describes the final state + expected response code/UI state, unambiguously"),
    ("J", "Related Edge ID", "E01-E20 / M01-M12 / A01-A07 or blank", "Mandatory traceability column — the bridge between functional cases and edge cases"),
    ("K", "Executor", "Enum", "AUTO-API · AUTO-WEB · PARTIAL · HUMAN — see the Legend table below"),
    ("L", "Execution Method", "Short text", "Concretizes column K, e.g. \"curl POST /v1/transactions/transfer\""),
    ("M", "Status", "Enum", "Not Run · Pass · Fail · Blocked · N/A — defaults to Not Run when the file is generated"),
    ("N", "Actual Result", "Text", "Filled in at execution — AUTO-API/AUTO-WEB: Claude fills in the real response"),
    ("O", "Evidence", "Text (path/link)", "Screenshot, response JSON log, or a cross-reference to TestEdge_X in CI"),
    ("P", "Executed By", "Text", "\"Claude (auto)\" or a person's name"),
    ("Q", "Last Run Date", "Date (yyyy-mm-dd)", "ISO format"),
    ("R", "Defect ID / Notes", "Text", "Link to the Jira/GitHub issue if Fail, or a free-form note"),
]

LEGEND_ROWS = [
    ("AUTO-API", "Claude runs everything itself via direct HTTP requests (curl/PowerShell) to the backend, reads the response and fills in Pass/Fail itself — no human needed",
     "Most of sheet 04 (platform), most of the E-series in sheet 05, and the \"behind the scenes\" part (state/response) even when the UI is mobile"),
    ("AUTO-WEB", "Claude runs everything itself through a browser automation tool on hpay-admin — click, read the DOM, take screenshots as Evidence",
     "All of sheet 03 (admin ops), the A-series in sheet 05"),
    ("PARTIAL", "Claude can confirm the correct backend behavior via API (did the money move correctly, is the state machine correct, is the response code correct) but can NOT confirm the mobile UI displays correctly — a human must review the UI part for the case to count as a full \"Pass\". HARD RULE: never auto-set Pass, only Blocked pending human UI confirmation.",
     "Most of sheets 01-02 (personal + business run on the mobile app) — Claude pre-fills the backend part and leaves the UI cell blank for a person to sign off"),
    ("HUMAN", "Needs a real device/simulator, hardware (a real camera for KYC, fingerprint/Face ID sensors), or visual observation that cannot be measured via API — Claude cannot touch any part of this case",
     "The M-series (biometrics, KYC camera, background/foreground), the KYC photo capture part of sheet 01, every assertion about \"the app displays correctly\" that cannot be observed via API"),
]


def style_header(ws):
    for col_idx, header in enumerate(HEADERS, start=1):
        cell = ws.cell(row=1, column=col_idx, value=header)
        cell.font = HEADER_FONT
        cell.fill = HEADER_FILL
        cell.alignment = Alignment(wrap_text=True, vertical="center", horizontal="center")
        cell.border = BORDER
    ws.row_dimensions[1].height = 30
    ws.freeze_panes = "A2"


def write_data_rows(ws, rows):
    for r_idx, data in enumerate(rows, start=2):
        for c_idx, value in enumerate(data, start=1):
            cell = ws.cell(row=r_idx, column=c_idx, value=value)
            cell.alignment = WRAP
            cell.border = BORDER
        ws.row_dimensions[r_idx].height = 60


def add_validations(ws, n_rows):
    def dv(formula_list, col_letter):
        formula = '"' + ",".join(formula_list) + '"'
        d = DataValidation(type="list", formula1=formula, allow_blank=True, showDropDown=False)
        ws.add_data_validation(d)
        d.add(f"{col_letter}2:{col_letter}{n_rows + 1}")
        return d

    dv(TEST_TYPES, "D")
    dv(PRIORITIES, "E")
    dv(EXECUTORS, "K")
    dv(STATUSES, "M")


def set_col_widths(ws):
    for i, w in enumerate(COL_WIDTHS, start=1):
        ws.column_dimensions[get_column_letter(i)].width = w


def build_readme_sheet(ws, file_title, case_count):
    ws.sheet_view.showGridLines = False
    ws.column_dimensions["A"].width = 8
    ws.column_dimensions["B"].width = 22
    ws.column_dimensions["C"].width = 34
    ws.column_dimensions["D"].width = 70

    title_font = Font(bold=True, size=16, color="0B5FFF")
    h2_font = Font(bold=True, size=13, color="111827")
    body_font = Font(size=10.5, color="374151")

    r = 1
    ws.cell(row=r, column=1, value=f"Read Me — {file_title}").font = title_font
    r += 1
    ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=4)
    c = ws.cell(row=r, column=1, value=f"{case_count} test cases · Main sheet \"Test Cases\" · Source: docs/qa/plan/implementation-plan.html §4/§6/§8")
    c.font = Font(italic=True, size=10, color="6B7280")
    c.alignment = Alignment(wrap_text=True)
    r += 2

    ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=4)
    intro_cell = ws.cell(row=r, column=1, value=README_INTRO)
    intro_cell.font = body_font
    intro_cell.alignment = Alignment(wrap_text=True, vertical="top")
    ws.row_dimensions[r].height = 45
    r += 2

    ws.cell(row=r, column=1, value="Column schema (A–R)").font = h2_font
    r += 1
    headers2 = ["Col", "Column name", "Type / Valid values", "Notes"]
    for i, h in enumerate(headers2, start=1):
        cell = ws.cell(row=r, column=i, value=h)
        cell.font = Font(bold=True, color="FFFFFF")
        cell.fill = HEADER_FILL
        cell.border = BORDER
    r += 1
    for col, name, kind, note in SCHEMA_ROWS:
        ws.cell(row=r, column=1, value=col).border = BORDER
        ws.cell(row=r, column=2, value=name).border = BORDER
        c3 = ws.cell(row=r, column=3, value=kind); c3.border = BORDER; c3.alignment = WRAP
        c4 = ws.cell(row=r, column=4, value=note); c4.border = BORDER; c4.alignment = WRAP
        ws.row_dimensions[r].height = 32
        r += 1
    r += 1

    ws.cell(row=r, column=1, value="Legend — Executor (column K)").font = h2_font
    r += 1
    headers3 = ["Label", "Meaning", "Applies to", ""]
    for i, h in enumerate(headers3[:3], start=1):
        cell = ws.cell(row=r, column=i, value=h)
        cell.font = Font(bold=True, color="FFFFFF")
        cell.fill = HEADER_FILL
        cell.border = BORDER
    ws.merge_cells(start_row=r, start_column=3, end_row=r, end_column=4)
    r += 1
    for label, meaning, applies in LEGEND_ROWS:
        c1 = ws.cell(row=r, column=1, value=label); c1.font = Font(bold=True); c1.border = BORDER
        c2 = ws.cell(row=r, column=2, value=meaning); c2.alignment = WRAP; c2.border = BORDER
        ws.merge_cells(start_row=r, start_column=3, end_row=r, end_column=4)
        c3 = ws.cell(row=r, column=3, value=applies); c3.alignment = WRAP; c3.border = BORDER
        ws.row_dimensions[r].height = 70
        r += 1
    r += 1

    ws.cell(row=r, column=1, value="Test ID convention").font = h2_font
    r += 1
    conv = ws.cell(row=r, column=1, value=(
        "TC-<GROUP>-<NNN>  — sheets 01-04, numbered sequentially within the group. GROUP ∈ {PER, BIZ, ADM, PLT}\n"
        "TC-EDG-<EDGE-ID>  — sheet 05, ID = the original edge-case ID itself (E01-E20 backend, M01-M12 mobile, A01-A07 admin)\n\n"
        "Two-way traceability: a functional case that touches a risk with an existing edge ID records exactly that ID in column J (no duplicate TC-EDG is created); "
        "each of the 37 original IDs has exactly 1 \"anchor\" row in 05-edge-cases.xlsx."
    ))
    ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=4)
    conv.alignment = Alignment(wrap_text=True, vertical="top")
    conv.font = body_font
    ws.row_dimensions[r].height = 90
    r += 2

    note = ws.cell(row=r, column=1, value=(
        "Hard rule: a case with Executor=PARTIAL must never be automatically marked Pass — only Blocked (pending human confirmation of the mobile UI) "
        "or Not Run. A case with Executor=HUMAN is not touched by Claude — it stays Not Run, with the reason recorded in column R."
    ))
    ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=4)
    note.font = Font(italic=True, size=10, color="DC2626")
    note.alignment = Alignment(wrap_text=True)
    ws.row_dimensions[r].height = 40


def build_workbook(file_title, rows, out_path):
    wb = Workbook()
    ws = wb.active
    ws.title = "Test Cases"
    style_header(ws)
    write_data_rows(ws, rows)
    add_validations(ws, len(rows))
    set_col_widths(ws)

    readme = wb.create_sheet("Read Me")
    build_readme_sheet(readme, file_title, len(rows))

    wb.save(out_path)
    print(f"Wrote {out_path} — {len(rows)} test cases")


def main(out_dir=None):
    """Generate all 5 workbooks. Optional out_dir (e.g. a temp dir) overrides OUT_DIR."""
    out_dir = out_dir or OUT_DIR
    os.makedirs(out_dir, exist_ok=True)
    build_workbook("01-personal-user.xlsx", sheet01_rows(), os.path.join(out_dir, "01-personal-user.xlsx"))
    build_workbook("02-business-maker-checker.xlsx", sheet02_rows(), os.path.join(out_dir, "02-business-maker-checker.xlsx"))
    build_workbook("03-admin-ops.xlsx", sheet03_rows(), os.path.join(out_dir, "03-admin-ops.xlsx"))
    build_workbook("04-platform-crosscutting.xlsx", sheet04_rows(), os.path.join(out_dir, "04-platform-crosscutting.xlsx"))
    build_workbook("05-edge-cases.xlsx", sheet05_rows(), os.path.join(out_dir, "05-edge-cases.xlsx"))
    print("Done. 5 files written to", out_dir)


if __name__ == "__main__":
    import sys
    main(sys.argv[1] if len(sys.argv) > 1 else None)
