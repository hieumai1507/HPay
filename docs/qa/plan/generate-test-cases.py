# -*- coding: utf-8 -*-
"""
Generate the 5 manual-QA Excel test-case workbooks for HPay.

Source of truth: docs/qa/plan/implementation-plan.html (schema, ID
convention, executor rules) and docs/qa/plan/solution-architecture.html.
Content sourced from:
  - docs/guide/functional-overview.vi.html (FEATURES array — 28 features)
  - docs/architecture/01-backend-solution-architecture.html (E01-E20)
  - docs/architecture/02-mobile-solution-architecture.html (M01-M12)
  - src/admin/docs/plans/e2e-playwright-suite/implementation-plan.html §8 (A01-A07)

Re-run with: python generate-test-cases.py
Output: D:\\hpay\\docs\\qa\\test-cases\\0{1..5}-*.xlsx
"""
import os
from openpyxl import Workbook
from openpyxl.styles import Font, Alignment, PatternFill, Border, Side
from openpyxl.utils import get_column_letter
from openpyxl.worksheet.datavalidation import DataValidation

OUT_DIR = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "test-cases"))

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
EXECUTORS = ["AUTO-API", "AUTO-WEB", "PARTIAL", "HUMAN"]

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
    "E01": ("Client gửi trùng request nạp tiền/chuyển tiền (double-tap / retry mạng)",
             "hpay-backend", "TestEdge_E01_IdempotentTopup",
             "Idempotency-Key UNIQUE — request thứ 2 với cùng key trả lại đúng kết quả cũ, không tạo giao dịch mới"),
    "E02": ("2 giao dịch cùng lúc rút từ 1 ví, tổng vượt số dư",
             "hpay-backend", "TestEdge_E02_ConcurrentOverdraw",
             "SELECT FOR UPDATE tuần tự hóa — đúng 1 giao dịch thành công, giao dịch còn lại bị từ chối do không đủ số dư"),
    "E03": ("A→B và B→A đồng thời (2 ví khóa chéo nhau)",
             "hpay-backend", "TestEdge_E03_CrossTransferNoDeadlock",
             "Lock ordering theo wallet_id — cả 2 giao dịch hoàn tất, không deadlock, không timeout"),
    "E04": ("Worker chết giữa chừng sau khi DEBIT xong, chưa CREDIT",
             "hpay-backend", "TestEdge_E04_RedeliveryNoDoubleSideEffect",
             "Redis Streams chưa XACK → XAUTOCLAIM giao worker khác; handler idempotent (check step_executions) — không DEBIT/CREDIT trùng"),
    "E05": ("DB commit xong nhưng publish event thất bại",
             "hpay-backend", "TestEdge_E05_OutboxSurvivesBrokerOutage",
             "Transactional outbox — event nằm trong DB, outbox relay publish lại khi broker hồi phục, không mất event"),
    "E06": ("Event bị publish 2 lần (relay retry)",
             "hpay-backend", "TestEdge_E06_DuplicateResultEventIdempotent",
             "Consumer check step_executions (instance, step, attempt) trước khi side-effect — event thứ 2 bị bỏ qua, không double-process"),
    "E07": ("CREDIT fail vĩnh viễn sau khi DEBIT thành công (ví nhận bị khóa giữa chừng)",
             "hpay-backend", "TestEdge_E07_CompensationReversesMovedFunds",
             "Saga compensation: post bút toán đảo, transaction chuyển sang failed, người gửi nhận notify hoàn tiền, số dư người gửi khôi phục đúng"),
    "E08": ("Checker A và B bấm approve cùng lúc ở tier chỉ cần 1 chữ ký",
             "hpay-backend", "TestEdge_E08_CheckerRaceSingleFinalize",
             "FOR UPDATE trên transaction_approvals + UNIQUE(approval_id, checker_id) — đúng 1 checker thắng và finalize, checker còn lại nhận thông báo lệnh đã được xử lý (không phải lỗi hệ thống)"),
    "E09": ("Admin sửa chính sách phê duyệt khi lệnh đang chờ duyệt",
             "hpay-backend", "TestEdge_E09_PolicySnapshotSurvivesPolicyChange",
             "policy_snapshot chụp tại thời điểm tạo lệnh — lệnh đang treo tiếp tục theo policy cũ, không bị ảnh hưởng bởi version policy mới"),
    "E10": ("Lệnh chi tổ chức chờ duyệt quá 24h",
             "hpay-backend", "TestEdge_E10_ExpiredApprovalFailsClean",
             "expires_at + cron quét → approval chuyển expired, workflow fail sạch, không có tiền bị treo lơ lửng"),
    "E11": ("OTP nhập sai 3 lần liên tiếp",
             "hpay-backend", "TestEdge_E11_OTPBruteForce",
             "attempts >= max_attempts → challenge hiện tại vô hiệu, phải xin challenge OTP mới; 5 challenge fail liên tiếp → khóa approve 15 phút"),
    "E12": ("User bị khóa (fraud) đúng lúc giao dịch đang ở giữa workflow",
             "hpay-backend", "TestEdge_E12_LockedUserFailsMidWorkflow",
             "Mọi step wallet re-check account_status trong cùng transaction dù step VALIDATE đã qua trước đó — giao dịch bị chặn giữa chừng, không có tiền di chuyển sau khi khóa"),
    "E13": ("Số tiền giao dịch bằng 0, âm, hoặc vượt hạn mức theo cấp KYC",
             "hpay-backend", "TestEdge_E13_LimitByKycTier",
             "Validator + CHECK constraint chặn ở tầng API lẫn DB — request bị từ chối rõ ràng, không tạo giao dịch"),
    "E14": ("Transfer đến chính mình (self-transfer)",
             "hpay-backend", "TestEdge_E14_SelfTransferRejected",
             "CHECK no_self_transfer + validate ở API — request bị từ chối trước khi chạm workflow engine"),
    "E15": ("Nạp tiền: bank callback (mock) đến 2 lần cho cùng 1 lần nạp",
             "hpay-backend", "TestEdge_E15_DuplicateBankCallback",
             "Idempotent theo ref_code sao kê — callback thứ 2 bị bỏ qua, ví chỉ được cộng tiền đúng 1 lần"),
    "E16": ("Clock skew / giao dịch cắt ngang nửa đêm khi đối soát",
             "hpay-backend", "TestEdge_E16_ReconCutoffNoFalseMismatch",
             "Recon so khớp theo ref_code chứ không chỉ theo ngày — giao dịch lệch cutoff được phân loại riêng, không báo mismatch giả"),
    "E17": ("Redis down toàn bộ trong lúc có giao dịch mới",
             "hpay-backend", "TestEdge_E17_RedisDownNoDataLoss",
             "API tạo transaction vẫn ghi DB + outbox thành công (202); engine dừng tiến độ chờ Redis hồi, không mất dữ liệu, không lỗi 500"),
    "E18": ("Migration mới làm hỏng schema hiện có",
             "hpay-backend", "CI job: migrate up + down + up trên DB sạch",
             "CI chặn merge nếu migrate down không revert sạch hoặc migrate up lần 2 lỗi — không migration hỏng nào lọt vào main"),
    "E19": ("Money request hết hạn đúng lúc người trả bấm \"Trả\" (race với sweeper)",
             "hpay-backend", "TestEdge_E19_ExpiryRacePay",
             "FOR UPDATE tuần tự hóa Pay() và sweeper.ExpireDue() — đúng 1 trong 2 thắng; nếu expired thắng thì Pay() trả ErrExpired, không tạo giao dịch \"mồ côi\" nào"),
    "E20": ("2 sweeper (recovery) chạy song song trên cùng 1 workflow instance đang kẹt",
             "hpay-backend", "TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim",
             "Lease claim bằng UPDATE ... SET status='recovering' WHERE ... RETURNING — chỉ 1 sweeper claim thành công, chỉ 1 lần re-emit sự kiện, instance không bị recover trùng"),

    "M01": ("User double-tap nút \"Chuyển tiền\"",
             "hpay-mobile", "TestEdge_M01_DoubleTapTransfer_SingleRequest",
             "Nút bị disable ngay khi tap + Idempotency-Key sinh 1 lần duy nhất lúc vào màn confirm (không sinh lại lúc bấm) — chỉ 1 giao dịch được tạo dù tap nhiều lần"),
    "M02": ("Request timeout / mất mạng ngay sau khi bấm chuyển tiền",
             "hpay-mobile", "TestEdge_M02_TransferPollTimeout_ProcessingCopy",
             "KHÔNG hiển thị \"thất bại\" vội — chuyển sang màn Processing, poll GET /transactions/{refCode} bằng chính idempotency retry; timeout poll 30s hiển thị \"Đang xử lý, kiểm tra lại ở Lịch sử\""),
    "M03": ("Transaction đang async, workflow chưa xử lý xong",
             "hpay-mobile", "TestEdge_M03_TransferFailed_ErrorMapDisplay",
             "Màn Processing poll theo backoff 1s→2s→4s; trạng thái pending_approval hiển thị copy riêng \"Chờ doanh nghiệp phê duyệt\" thay vì báo lỗi chung chung"),
    "M04": ("Access token hết hạn giữa lúc đang thao tác (giữa flow)",
             "hpay-mobile", "TestEdge_M04_Refresh_SingleFlight_RetryOnce",
             "Interceptor tự refresh theo cơ chế single-flight (chỉ 1 refresh request, các request khác queue chờ); refresh fail → điều hướng về Login nhưng GIỮ nguyên draft form đang nhập"),
    "M05": ("Refresh token bị revoke (đăng nhập ở máy khác / theft detection)",
             "hpay-mobile", "TestEdge_M05_RefreshRevoked_ForceLogout",
             "Force logout ngay + thông báo rõ lý do (không phải lỗi mạng chung chung) — toàn bộ phiên trên thiết bị cũ bị thu hồi"),
    "M06": ("Nhập số tiền lớn / có định dạng lạ trong ô nhập liệu",
             "hpay-mobile", "TestEdge_M06_BigIntMoney_NoPrecisionLoss",
             "Chỉ nhận integer VND, format ngăn cách nghìn khi gõ, chặn paste ký tự lạ; parse bằng string→BigInt — không bao giờ đi qua float JS, không mất độ chính xác với số tiền lớn"),
    "M07": ("App chuyển sang background lúc đang ở màn OTP/confirm",
             "hpay-mobile", "TestEdge_M07_AppBackground_PrivacyOverlay",
             "Che mờ toàn bộ nội dung app bằng Native Blur khi vào Task Switcher (giống VCB Digibank); OTP còn TTL thì giữ nguyên, hết hạn thì báo xin lại khi quay lại app"),
    "M08": ("Deep link \"duyệt lệnh chi\" mở khi user chưa đăng nhập",
             "hpay-mobile", "TestEdge_M08_DeepLink_PendingAuth",
             "Lưu pending route vào store trước khi vào màn Login; sau khi login thành công, điều hướng thẳng tới đúng màn duyệt lệnh đã click từ deep link"),
    "M09": ("Checker mở 1 lệnh đã bị người khác duyệt xong/từ chối trước đó",
             "hpay-mobile", "TestEdge_M09 (mô tả trong 02-mobile-solution-architecture.html — chưa có unit test riêng, xác nhận qua UI thật)",
             "UI xử lý ERR_ALREADY_FINALIZED: tự refresh hộp thư (inbox), hiển thị toast báo đúng trạng thái mới của lệnh, không crash và không báo lỗi mạng chung chung"),
    "M10": ("Ảnh CCCD/selfie chụp bị mờ hoặc quá tối",
             "hpay-mobile", "TestEdge_M10_KYCImageHeuristic_Validation",
             "Kiểm tra client-side (blur/brightness heuristic) trước khi upload, hướng dẫn chụp lại ngay tại chỗ — giảm vòng lặp bị AI/nhân viên reject sau khi đã upload"),
    "M11": ("Upload ảnh KYC bị rớt mạng giữa chừng",
             "hpay-mobile", "TestEdge_M11_KYCUpload_NetworkDropResume",
             "2 ảnh (giấy tờ + selfie) upload tách phần, resume theo request id khi có mạng lại; trạng thái draft được lưu local, không phải chụp lại từ đầu"),
    "M12": ("Danh sách lịch sử giao dịch dài, cuộn liên tục",
             "hpay-mobile", "TestEdge_M12_HistoryFlashListPagination",
             "Cursor pagination + FlashList; pull-to-refresh không làm vị trí cuộn nhảy lung tung, không load trùng/lặp item"),

    "A01": ("2 admin (hoặc 2 checker) cùng bấm \"Duyệt\" 1 task trong My Tasks gần như đồng thời",
             "hpay-admin", "TestE2E_MyTasks_TwoCheckersRaceOneWins (kế hoạch trong e2e-playwright-suite, chưa triển khai)",
             "1 context nhận 200 + row cập nhật thành công; context còn lại nhận lỗi 409 ERR_ALREADY_FINALIZED kèm toast rõ ràng, row tự cập nhật theo state mới tại chỗ — không phải reload cả bảng"),
    "A02": ("Trang bị chèn thêm instance mới trong lúc reviewer đang ở trang phân trang sau (Workflow Monitor)",
             "hpay-admin", "TestE2E_Workflow_PaginationStableUnderInsert (kế hoạch trong e2e-playwright-suite, chưa triển khai)",
             "Cursor pagination (không phải offset) — next/prev không làm trôi hàng đang xem dù có workflow instance mới được tạo song song"),
    "A03": ("Phiên admin (JWT) hết hạn giữa lúc đang review, có filter/search param trên URL",
             "hpay-admin", "TestE2E_SessionExpiry_ReLoginKeepsFilters (kế hoạch trong e2e-playwright-suite, chưa triển khai)",
             "Action tiếp theo sau khi token hết hạn hiện modal re-login (không văng thẳng về trang trắng); sau khi login lại, quay về đúng URL + filter/search param cũ, không mất ngữ cảnh đang xem"),
    "A04": ("URL ảnh KYC (signed URL) hết hạn khi admin đang xem chi tiết hồ sơ",
             "hpay-admin", "TestE2E_KYCDetail_ImageURLExpiryRetry (kế hoạch trong e2e-playwright-suite, chưa triển khai)",
             "FE nhận lỗi 403 ở request ảnh, tự động xin URL mới đúng 1 lần và hiển thị lại ảnh — chỉ báo lỗi cho người dùng nếu lần retry đó cũng thất bại"),
    "A05": ("2 tab trình duyệt cùng mở màn sửa chính sách (policy) của 1 tổ chức",
             "hpay-admin", "TestE2E_Orgs_TwoTabsEditPolicy (kế hoạch trong e2e-playwright-suite, chưa triển khai)",
             "Tab lưu sau nhận lỗi 409 (conflict version), xử lý qua interceptor chung với toast thống nhất, không crash trang, không ghi đè âm thầm policy tab kia vừa lưu"),
    "A06": ("Admin bấm xuất báo cáo đối soát ra Excel (recon export)",
             "hpay-admin", "TestE2E_Recon_ExportDoesNotBlockUI (kế hoạch trong e2e-playwright-suite, chưa triển khai)",
             "UI vẫn tương tác được bình thường trong lúc chờ export (không có spinner full-page chặn toàn màn hình); hệ thống poll tới khi có link tải xong rồi mới hiện nút tải"),
    "A07": ("Admin mở chi tiết 1 bước workflow (step detail) có chứa dữ liệu cá nhân (PII) trong JSON input/output",
             "hpay-admin", "TestE2E_WorkflowDetail_StepJSONMasked (kế hoạch trong e2e-playwright-suite, chưa triển khai)",
             "Cả network response lẫn DOM hiển thị đều đã che (***) các trường PII nhạy cảm — không lộ raw PII qua DevTools/response ngay cả với tài khoản admin hợp lệ"),
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

    f = "Đăng ký &amp; đăng nhập"
    rows += [
        row("TC-PER-001", f, R, "Happy Path", "P0",
            "Số điện thoại chưa từng đăng ký trên hệ thống",
            "1. Mở app, chọn Đăng ký\n2. Nhập số điện thoại + đặt mật khẩu\n3. Xác nhận OTP đăng ký\n4. Hệ thống cấp access token + refresh token, vào thẳng Home",
            "SĐT 09xxxxxxx01, mật khẩu Test@1234",
            "Đăng ký thành công, access token TTL ngắn + refresh token TTL dài được lưu, app vào Home không cần đăng nhập lại",
            "", "PARTIAL", "curl POST /v1/auth/register + /v1/auth/login xác minh token; UI mobile cần người xác nhận màn hình chào mừng"),
        row("TC-PER-002", f, R, "Negative", "P1",
            "Tài khoản đã tồn tại với mật khẩu đã biết",
            "1. Mở màn Đăng nhập\n2. Nhập đúng SĐT, sai mật khẩu 1 lần\n3. Quan sát thông báo lỗi",
            "SĐT đã đăng ký, mật khẩu sai",
            "Từ chối đăng nhập với thông báo rõ ràng \"sai số điện thoại hoặc mật khẩu\", không cấp token, không lộ chi tiết field nào sai",
            "", "PARTIAL", "curl POST /v1/auth/login với mật khẩu sai, xác nhận response 401"),
        row("TC-PER-003", f, R, "Edge Case", "P0",
            "Tài khoản đã tồn tại, đang ở màn nhập OTP xác thực (đăng ký hoặc bước cần OTP)",
            "1. Nhập sai mã OTP 3 lần liên tiếp\n2. Quan sát trạng thái challenge\n3. Thử nhập đúng OTP cũ lần thứ 4",
            "OTP sai bất kỳ x3, sau đó OTP đúng của challenge cũ",
            "Sau lần sai thứ 3, challenge hiện tại bị vô hiệu — nhập đúng OTP cũ ở lần 4 vẫn bị từ chối, phải xin OTP mới",
            "E11", "PARTIAL", "curl POST /v1/auth/otp/verify x4 lần, xác nhận response sau lần 3"),
        row("TC-PER-004", f, R, "Edge Case", "P0",
            "User đã đăng nhập trên thiết bị A, có refresh token hợp lệ",
            "1. Đăng nhập cùng tài khoản trên thiết bị B (giả lập bằng client khác)\n2. Quay lại thiết bị A, thử dùng refresh token cũ để làm mới access token",
            "1 tài khoản, 2 phiên đăng nhập (device A, device B)",
            "Hệ thống phát hiện refresh token cũ bị dùng lại sau khi đã có phiên mới hơn (theft detection) → toàn bộ phiên trên mọi thiết bị bị thu hồi, thiết bị A bị force logout",
            "M05", "PARTIAL", "curl POST /v1/auth/refresh bằng token thiết bị A sau khi thiết bị B đã refresh — xác nhận 401 + session revoked"),
    ]

    f = "Xác minh danh tính (KYC)"
    rows += [
        row("TC-PER-005", f, R, "Happy Path", "P0",
            "User đã đăng ký, chưa xác minh KYC",
            "1. Mở màn Xác minh danh tính\n2. Chụp ảnh CCCD/CMND\n3. Chụp ảnh selfie đối chiếu khuôn mặt\n4. Chờ AI (Claude vision) chấm điểm",
            "Ảnh CCCD rõ nét + ảnh selfie rõ nét, điểm AI mô phỏng ở mức cao",
            "AI trả điểm tin cậy cao → tự động duyệt ngay, hạn mức giao dịch/ngày của user được nâng lên đúng theo cấp KYC mới",
            "", "HUMAN", "Cần camera thật để chụp CCCD + selfie — không mô phỏng được qua API/browser tool"),
        row("TC-PER-006", f, R, "Negative", "P1",
            "User đã đăng ký, chưa xác minh KYC",
            "1. Chụp ảnh CCCD/selfie ở chất lượng cố ý kém (ảnh không khớp, giấy tờ giả lập)\n2. Chờ AI chấm điểm",
            "Ảnh không đạt chuẩn / không khớp khuôn mặt",
            "AI trả điểm thấp → tự động từ chối, hiển thị lý do rõ ràng, user được hướng dẫn nộp lại",
            "", "HUMAN", "Cần camera thật; phần backend (workflow kyc_verification reject) có thể verify song song qua API"),
        row("TC-PER-007", f, R, "Edge Case", "P1",
            "User đang ở bước chụp ảnh CCCD/selfie",
            "1. Chụp ảnh trong điều kiện thiếu sáng / rung tay gây mờ\n2. Quan sát phản hồi trước khi ảnh được upload lên server",
            "Ảnh mờ/tối cố ý",
            "Client-side heuristic (blur/brightness) chặn ngay tại app, yêu cầu chụp lại — không tốn round-trip upload lên server cho ảnh chắc chắn sẽ bị reject",
            "M10", "HUMAN", "Cần camera thật + điều kiện ánh sáng thực tế để trigger heuristic"),
        row("TC-PER-008", f, R, "Edge Case", "P1",
            "User đang giữa quá trình upload 2 ảnh KYC (giấy tờ + selfie)",
            "1. Bắt đầu upload ảnh giấy tờ\n2. Ngắt mạng giữa chừng (bật chế độ máy bay)\n3. Bật lại mạng, quan sát app có tự resume không",
            "2 ảnh KYC hợp lệ, ngắt mạng ở giữa lần upload ảnh thứ nhất",
            "Upload tách phần theo request id, resume đúng từ điểm dở dang khi có mạng lại; trạng thái draft được lưu local, không phải chụp lại từ đầu",
            "M11", "HUMAN", "Cần thiết bị thật để bật/tắt mạng giữa luồng upload thực tế"),
    ]

    f = "Nạp tiền vào ví"
    rows += [
        row("TC-PER-009", f, R, "Happy Path", "P0",
            "User đã đăng nhập, có ví hoạt động",
            "1. Chọn số tiền cần nạp\n2. Xác thực bằng OTP gửi về điện thoại\n3. Chờ ngân hàng (mock) xác nhận",
            "Số tiền 500,000 VND, OTP mock hợp lệ",
            "Ngân hàng mock xác nhận thành công, ví được cộng đúng 500,000 VND ngay, có bản ghi ledger double-entry tương ứng",
            "", "PARTIAL", "curl POST /v1/wallet/topup + xác nhận số dư qua GET /v1/wallet — UI mobile cần người xác nhận animation/thông báo"),
        row("TC-PER-010", f, R, "Edge Case", "P0",
            "User vừa bấm nạp tiền, app bị treo/mất mạng ngay sau khi request đi (chưa nhận response)",
            "1. Gửi request nạp tiền với Idempotency-Key X\n2. Giả lập mất mạng/app treo, không nhận được response\n3. User bấm nạp lại — app gửi lại đúng request với cùng key X",
            "Idempotency-Key X, cùng số tiền, gửi 2 lần",
            "Lần gửi lại thứ 2 với cùng Idempotency-Key trả về đúng kết quả giao dịch gốc, ví KHÔNG bị cộng tiền 2 lần",
            "E01", "PARTIAL", "curl POST /v1/wallet/topup 2 lần với cùng Idempotency-Key header, so sánh response + số dư trước/sau"),
        row("TC-PER-011", f, R, "Edge Case", "P0",
            "User đã tạo 1 yêu cầu nạp tiền đang chờ ngân hàng xác nhận",
            "1. Giả lập bank callback (mock) gọi vào hệ thống 2 lần cho cùng 1 ref_code sao kê\n2. Kiểm tra số dư ví sau cả 2 lần callback",
            "1 ref_code sao kê, gọi callback endpoint 2 lần liên tiếp",
            "Callback thứ 2 bị nhận diện trùng theo ref_code và bị bỏ qua — ví chỉ được cộng tiền đúng 1 lần",
            "E15", "PARTIAL", "curl POST vào endpoint bank-callback mock 2 lần cùng ref_code, xác nhận số dư ví chỉ tăng 1 lần"),
    ]

    f = "Chuyển tiền P2P"
    rows += [
        row("TC-PER-012", f, R, "Happy Path", "P0",
            "User A và User B đều có ví hoạt động, User A đủ số dư",
            "1. User A nhập SĐT User B + số tiền\n2. Xem trước phí giao dịch (nếu có)\n3. Xác nhận bằng PIN\n4. Theo dõi màn hình tiến trình theo thời gian thực",
            "Số tiền 100,000 VND, SĐT User B hợp lệ, PIN đúng",
            "Giao dịch chạy qua workflow engine, hoàn tất thành công; User A bị trừ đúng số tiền + phí, User B được cộng đúng số tiền, cả 2 nhận thông báo",
            "", "PARTIAL", "curl POST /v1/transactions/transfer + xác nhận số dư 2 ví qua API — UI theo dõi tiến trình cần người xác nhận"),
        row("TC-PER-013", f, R, "Edge Case", "P0",
            "User A đủ số dư, đang ở màn xác nhận chuyển tiền",
            "1. Bấm nút \"Chuyển tiền\" nhanh 2 lần liên tiếp (double-tap)\n2. Kiểm tra số lượng giao dịch được tạo và số dư sau đó",
            "Số tiền 50,000 VND, double-tap trong <1 giây",
            "Nút bị disable ngay khi tap đầu tiên + Idempotency-Key chỉ sinh 1 lần cho cả flow confirm — chỉ đúng 1 giao dịch được tạo, ví chỉ bị trừ 1 lần",
            "M01", "PARTIAL", "curl POST /v1/transactions/transfer 2 lần với cùng Idempotency-Key mô phỏng double-tap ở tầng backend"),
        row("TC-PER-014", f, R, "Edge Case", "P0",
            "User A đang gửi yêu cầu chuyển tiền",
            "1. Bấm chuyển tiền, giả lập mất mạng ngay sau khi request đã rời thiết bị (chưa nhận response)\n2. Quan sát màn hình có báo \"thất bại\" ngay không\n3. App tự poll trạng thái giao dịch theo refCode",
            "Số tiền bất kỳ, timeout mạng giả lập ngay sau request",
            "KHÔNG hiển thị \"thất bại\" vội — chuyển sang màn Processing, poll GET /transactions/{refCode}; nếu poll quá 30s vẫn chưa xong, hiển thị \"Đang xử lý, kiểm tra lại ở Lịch sử\" thay vì báo lỗi sai",
            "M02", "PARTIAL", "curl POST transfer rồi GET /v1/transactions/{refCode} lặp lại mô phỏng polling — UI Processing cần người xác nhận"),
        row("TC-PER-015", f, R, "Edge Case", "P1",
            "User A đủ số dư lớn (đủ để test số tiền cao)",
            "1. Nhập số tiền lớn (ví dụ 999,999,999 VND) vào ô nhập\n2. Xác nhận số hiển thị đúng từng đồng, không làm tròn/mất chính xác\n3. Thử paste ký tự không phải số vào ô nhập",
            "999,999,999 VND và chuỗi ký tự lạ dán vào ô nhập",
            "Ô nhập chỉ chấp nhận integer VND, format ngăn cách nghìn khi gõ, chặn ký tự lạ khi paste; số tiền được xử lý bằng string→BigInt xuyên suốt, không đi qua float JS, không sai lệch dù số lớn",
            "M06", "PARTIAL", "curl POST /v1/transactions/transfer với amount minor-unit lớn, xác nhận response amount khớp tuyệt đối bằng BIGINT"),
        row("TC-PER-016", f, R, "Edge Case", "P0",
            "User A có ví hoạt động",
            "1. Nhập chính số điện thoại của mình làm người nhận\n2. Nhập số tiền bất kỳ\n3. Bấm xác nhận",
            "SĐT người nhận = SĐT người gửi",
            "Request bị từ chối rõ ràng ngay từ tầng API (CHECK no_self_transfer), không tạo giao dịch, không trừ tiền",
            "E14", "AUTO-API", "curl POST /v1/transactions/transfer với recipient_phone = sender_phone, xác nhận response 400/422"),
    ]

    f = "Yêu cầu thanh toán &amp; Chia hoá đơn"
    rows += [
        row("TC-PER-017", f, R, "Happy Path", "P0",
            "User A và User B đều có ví hoạt động",
            "1. User A chọn User B, nhập số tiền + lý do\n2. Gửi yêu cầu — User B nhận thông báo\n3. User B bấm \"Trả\" trong hạn 72 giờ\n4. Kiểm tra trạng thái cả 2 phía",
            "Số tiền 200,000 VND, lý do \"Ăn trưa\"",
            "Yêu cầu chuyển sang trạng thái \"đã trả\" sau khi User B trả, tiền chuyển đi đúng như 1 giao dịch P2P bình thường, User A nhận thông báo \"B đã trả bạn\"",
            "", "PARTIAL", "curl POST /v1/money-requests + POST /v1/money-requests/{id}/pay, xác nhận số dư 2 ví"),
        row("TC-PER-018", f, R, "Negative", "P1",
            "Có 1 yêu cầu thanh toán đang chờ trả",
            "1. Không thao tác gì cho tới khi vượt quá hạn 72 giờ\n2. Kiểm tra trạng thái yêu cầu",
            "Yêu cầu có expires_at đã set lùi về quá khứ để giả lập hết hạn",
            "Yêu cầu tự chuyển sang trạng thái expired, người trả không còn bấm \"Trả\" được nữa",
            "", "AUTO-API", "curl UPDATE expires_at về quá khứ (qua tool DB test) rồi POST pay, xác nhận response từ chối do hết hạn"),
        row("TC-PER-019", f, R, "Edge Case", "P0",
            "Có 1 yêu cầu thanh toán sắp hết hạn, sweeper định kỳ đang chạy",
            "1. Đặt expires_at của yêu cầu về đúng thời điểm hiện tại (race)\n2. Gọi đồng thời Pay() và để sweeper.ExpireDue() chạy\n3. Kiểm tra kết quả và số giao dịch được tạo",
            "1 money request, expires_at = now(), 2 luồng chạy song song",
            "FOR UPDATE tuần tự hóa đúng 1 trong 2 thắng — nếu expired thắng, Pay() trả ErrExpired và KHÔNG có giao dịch nào được tạo (không có transaction mồ côi)",
            "E19", "AUTO-API", "curl POST /v1/money-requests/{id}/pay đồng thời với trigger sweeper qua script test, đếm số transactions tạo ra"),
        row("TC-PER-020", f, R, "Negative", "P1",
            "User A tạo 1 nhóm chia hoá đơn cho 3 người",
            "1. Tạo yêu cầu chia hoá đơn cho 3 người với tổng số tiền cố định\n2. 1 người trả, 2 người chưa trả\n3. Mở màn theo dõi tiến độ nhóm\n4. Bấm \"nhắc\" người còn thiếu",
            "Tổng hoá đơn 900,000 VND / 3 người = 300,000 VND/người",
            "Màn theo dõi hiển thị đúng avatar người đã trả (sáng rõ) và người chưa trả (mờ), thanh tiến độ đúng 300,000/900,000 (33%), nút \"nhắc\" gửi lại thông báo cho người chưa trả",
            "", "PARTIAL", "curl POST /v1/money-requests (split group) + kiểm tra progress qua API — UI progress bar cần người xác nhận"),
    ]

    f = "Nhận tiền qua QR"
    rows += [
        row("TC-PER-021", f, R, "Happy Path", "P0",
            "User A (chủ ví) và User B (người trả) đều có ví hoạt động",
            "1. User A mở màn QR của tôi, không nhập số tiền\n2. User B quét mã bằng camera app\n3. App B điền sẵn thông tin người nhận, User B tự nhập số tiền và xác nhận PIN",
            "QR không gắn sẵn số tiền, User B tự nhập 150,000 VND",
            "App B điền sẵn đúng thông tin người nhận (User A) từ mã QR; sau khi xác nhận PIN, tiền chuyển thành công tới đúng User A",
            "", "HUMAN", "Cần camera thật để quét QR — không mô phỏng qua API/browser tool"),
        row("TC-PER-022", f, R, "Negative", "P1",
            "User A tạo QR có gắn sẵn số tiền cụ thể",
            "1. User A tạo QR với số tiền cố định 50,000 VND\n2. User B quét mã\n3. Kiểm tra ô số tiền trên màn hình User B có sửa được không",
            "QR gắn sẵn 50,000 VND",
            "Form chuyển tiền của User B điền sẵn đúng 50,000 VND và không cho sửa số tiền — chỉ cần xác nhận PIN để chuyển",
            "", "HUMAN", "Cần camera thật để quét QR có gắn sẵn số tiền"),
        row("TC-PER-023", f, R, "Negative", "P1",
            "Có 1 mã QR đã hết hiệu lực hoặc bị chỉnh sửa payload",
            "1. Quét mã QR không hợp lệ (đã hết hạn ký số hoặc payload bị sửa)\n2. Quan sát phản hồi của app",
            "QR đã hết hạn chữ ký số / payload bị chỉnh sửa",
            "App từ chối rõ ràng \"mã QR không hợp lệ hoặc đã hết hạn\", không điền form chuyển tiền, không cho tiếp tục",
            "", "HUMAN", "Cần camera thật để quét mã QR test không hợp lệ"),
    ]

    f = "Hũ tiết kiệm (Vaults) &amp; làm tròn tự động"
    rows += [
        row("TC-PER-024", f, R, "Happy Path", "P1",
            "User đã đăng nhập, có ví hoạt động",
            "1. Tạo hũ tiết kiệm mới, đặt tên + mục tiêu số tiền\n2. Bỏ tiền vào hũ\n3. Rút bớt tiền ra khỏi hũ\n4. Kiểm tra sổ ghi của cả 2 thao tác",
            "Mục tiêu 5,000,000 VND, bỏ vào 200,000 VND, rút ra 50,000 VND",
            "Mỗi thao tác bỏ/rút đều tạo 1 giao dịch có ghi sổ (ledger) riêng, số dư hũ và số dư ví chính cập nhật đúng, khớp tuyệt đối",
            "", "PARTIAL", "curl POST /v1/vaults + /v1/vaults/{id}/deposit + /withdraw, xác nhận ledger qua API"),
        row("TC-PER-025", f, R, "Negative", "P2",
            "User có hũ tiết kiệm đã bật chế độ làm tròn",
            "1. Bật \"làm tròn\" cho 1 hũ\n2. Thực hiện 1 giao dịch chuyển tiền P2P với số tiền lẻ (không tròn nghìn)\n3. Kiểm tra hũ có được cộng đúng phần dư làm tròn không",
            "Chuyển 123,700 VND → làm tròn lên 124,000, phần dư 300 VND vào hũ",
            "Sau giao dịch P2P, hũ tiết kiệm được tự động cộng đúng 300 VND phần dư làm tròn, không sai lệch",
            "", "PARTIAL", "curl POST transfer với số tiền lẻ + GET /v1/vaults/{id} kiểm tra số dư hũ sau giao dịch"),
        row("TC-PER-026", f, R, "Negative", "P2",
            "User có hũ tiết kiệm với số dư nhất định",
            "1. Thử rút số tiền lớn hơn số dư hiện có trong hũ\n2. Quan sát phản hồi",
            "Số dư hũ 100,000 VND, thử rút 500,000 VND",
            "Request bị từ chối rõ ràng do không đủ số dư trong hũ, không tạo giao dịch, số dư hũ không đổi",
            "", "AUTO-API", "curl POST /v1/vaults/{id}/withdraw vượt số dư, xác nhận response 400/422"),
    ]

    f = "Lịch sử &amp; dòng thời gian giao dịch"
    rows += [
        row("TC-PER-027", f, R, "Happy Path", "P1",
            "User đã có ít nhất vài giao dịch trong lịch sử",
            "1. Mở danh sách lịch sử, lọc theo thời gian/loại giao dịch\n2. Bấm vào 1 giao dịch để xem chi tiết\n3. Xem dòng thời gian từng bước xử lý",
            "User có tối thiểu 3 giao dịch các loại khác nhau (topup, transfer, request)",
            "Danh sách lọc đúng theo tiêu chí; màn chi tiết hiển thị đầy đủ dòng thời gian từng bước xử lý (không chỉ thành công/thất bại) kèm thời điểm mỗi bước",
            "", "PARTIAL", "curl GET /v1/transactions?filter=... + GET /v1/transactions/{id}/timeline, đối chiếu dữ liệu — UI cần người xác nhận hiển thị"),
        row("TC-PER-028", f, R, "Negative", "P2",
            "User đã đăng nhập, đang ở trang chủ",
            "1. Mở app tới trang chủ, quan sát ô hiển thị số dư\n2. Bấm vào ô số dư để hiện tạm thời",
            "Không cần dữ liệu đặc biệt",
            "Số dư ví mặc định bị ẩn (che bằng dấu ***/chấm) khi mở app; sau khi bấm, số dư hiện ra tạm thời rồi tự ẩn lại sau khi rời màn hoặc timeout",
            "", "HUMAN", "Hành vi UI thuần tuý (ẩn/hiện số dư) — cần người quan sát trực tiếp trên app thật"),
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

    f = "Tạo tổ chức &amp; mời thành viên"
    rows += [
        row("TC-BIZ-001", f, OADMIN, "Happy Path", "P0",
            "User đã có tài khoản cá nhân, muốn tạo ví doanh nghiệp",
            "1. Tạo tổ chức, đặt tên\n2. Mời thành viên bằng số điện thoại\n3. Gán vai trò: maker / checker (kèm level)\n4. Kiểm tra thành viên xuất hiện đúng quyền",
            "Tên tổ chức, SĐT 2 thành viên (1 maker, 1 checker L1)",
            "Tổ chức được tạo, thành viên được mời xuất hiện trong danh sách với đúng vai trò và cấp checker đã gán",
            "", "PARTIAL", "curl POST /v1/orgs + /v1/orgs/{id}/members, xác nhận role qua API — UI mời thành viên cần người xác nhận"),
        row("TC-BIZ-002", f, OADMIN, "Negative", "P1",
            "Đang ở màn mời thành viên tổ chức",
            "1. Nhập số điện thoại chưa từng đăng ký HPay\n2. Gửi lời mời\n3. Quan sát trạng thái lời mời",
            "SĐT chưa tồn tại trên hệ thống",
            "Hệ thống xử lý rõ ràng: lời mời ở trạng thái chờ (pending) cho tới khi SĐT đó đăng ký, không lỗi crash, không mời thành công giả",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/members với SĐT chưa đăng ký, xác nhận response/trạng thái pending"),
        row("TC-BIZ-003", f, OADMIN, "Negative", "P2",
            "Tổ chức chưa có policy phê duyệt nào được thiết lập",
            "1. Gán 1 thành viên vai trò checker level 2\n2. Kiểm tra cảnh báo khi chưa có policy tương ứng cấp đó",
            "Checker level 2, tổ chức chưa có tier policy nào dùng L2",
            "Hệ thống cảnh báo rõ ràng cho Org Admin rằng checker L2 chưa có policy nào áp dụng, tránh hiểu lầm checker đã sẵn sàng hoạt động",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/policies đối chiếu level được gán — UI cảnh báo cần người xác nhận"),
    ]

    f = "Thiết lập chính sách phê duyệt"
    rows += [
        row("TC-BIZ-004", f, OADMIN, "Happy Path", "P0",
            "Tổ chức đã có ít nhất 1 checker L1 và 1 checker L2",
            "1. Đặt tên chính sách, khoảng số tiền áp dụng\n2. Định nghĩa tier 1 (dưới 5tr → 1 chữ ký L1) và tier 2 (trên 5tr → L1+L2)\n3. Lưu chính sách",
            "Tier 1: <5,000,000 VND / 1 chữ ký L1; Tier 2: ≥5,000,000 VND / L1+L2",
            "Chính sách được lưu thành công với version mới, áp dụng ngay cho lệnh chi tạo sau thời điểm lưu",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/policies, xác nhận cấu trúc tier qua API"),
        row("TC-BIZ-005", f, OADMIN, "Negative", "P1",
            "Tổ chức có 1 lệnh chi đang chờ duyệt theo policy hiện hành",
            "1. Trong lúc lệnh đang chờ duyệt, Org Admin sửa policy (đổi ngưỡng tier)\n2. Kiểm tra lệnh đang treo có bị đổi yêu cầu chữ ký không",
            "1 lệnh chi pending, policy version cũ vs version mới",
            "Lệnh đang chờ duyệt tiếp tục theo policy phiên bản cũ (giữ nguyên yêu cầu chữ ký ban đầu); chỉ lệnh tạo mới mới áp dụng policy vừa sửa",
            "E09", "PARTIAL", "curl PUT /v1/orgs/{id}/policies rồi GET lệnh cũ, xác nhận policy_snapshot không đổi"),
        row("TC-BIZ-006", f, OADMIN, "Edge Case", "P0",
            "Maker chuẩn bị tạo 1 lệnh chi ngay lúc Org Admin đang sửa policy",
            "1. Kích hoạt đồng thời: tạo lệnh chi (đọc policy hiện hành) và sửa policy (ghi version mới)\n2. Kiểm tra policy_snapshot được gắn vào lệnh vừa tạo",
            "1 lệnh chi tạo đồng thời với 1 policy update",
            "policy_snapshot của lệnh chi chụp đúng đúng 1 policy tại thời điểm tạo — không bị lẫn giữa version cũ/mới do race condition",
            "E09", "AUTO-API", "curl POST tạo lệnh chi song song với PUT policy qua script test, kiểm tra policy_snapshot của lệnh"),
    ]

    f = "Tạo lệnh chi"
    rows += [
        row("TC-BIZ-007", f, MAKER, "Happy Path", "P0",
            "Maker thuộc tổ chức đã có policy phê duyệt",
            "1. Chọn người nhận + số tiền + lý do chi\n2. Hệ thống tự tra chính sách phù hợp\n3. Kiểm tra trạng thái lệnh vừa tạo",
            "Số tiền 3,000,000 VND (thuộc tier 1)",
            "Lệnh được tạo ở trạng thái WAIT_APPROVAL, đúng tier 1 chữ ký được áp dụng, checker liên quan nhận được thông báo",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/disbursements, xác nhận policy_snapshot và trạng thái qua API"),
        row("TC-BIZ-008", f, MAKER, "Negative", "P0",
            "Maker thuộc tổ chức đã có policy phê duyệt",
            "1. Thử tạo lệnh chi với số tiền 0 hoặc số âm\n2. Quan sát phản hồi",
            "Số tiền 0 và -100,000",
            "Cả 2 request đều bị từ chối ở tầng validate, không tạo lệnh chi nào",
            "", "AUTO-API", "curl POST /v1/orgs/{id}/disbursements với amount=0 và amount âm, xác nhận response 400"),
        row("TC-BIZ-009", f, MAKER, "Negative", "P1",
            "Maker thuộc tổ chức đã có policy phê duyệt",
            "1. Nhập người nhận không tồn tại trên hệ thống (SĐT không hợp lệ)\n2. Xác nhận tạo lệnh",
            "SĐT người nhận không tồn tại",
            "Request bị từ chối rõ ràng, không tạo lệnh chi treo cho người nhận không xác định được",
            "", "AUTO-API", "curl POST /v1/orgs/{id}/disbursements với recipient không hợp lệ, xác nhận response lỗi"),
    ]

    f = "Theo dõi trạng thái lệnh chi"
    rows += [
        row("TC-BIZ-010", f, MAKER, "Happy Path", "P1",
            "Maker có 1 lệnh chi tier 2 (cần L1+L2) đang được ký dần",
            "1. Mở màn \"Trạng thái lệnh\" ngay sau khi tạo\n2. Theo dõi từng mốc: tạo lệnh → ký L1 → ký L2 → chuyển tiền",
            "Lệnh tier 2, 2 checker ký lần lượt",
            "Mỗi mốc hiển thị đúng thời gian cụ thể và tên người đã ký; khi đủ chữ ký, tiền tự động chuyển không cần thao tác thêm từ maker",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id}/timeline sau mỗi lần ký, đối chiếu mốc thời gian"),
        row("TC-BIZ-011", f, MAKER, "Negative", "P1",
            "Maker có 1 lệnh chi bị 1 checker từ chối",
            "1. Checker từ chối lệnh\n2. Maker mở màn trạng thái lệnh",
            "1 checker bấm Từ chối kèm lý do",
            "Màn trạng thái hiển thị rõ lệnh bị từ chối, kèm tên checker và lý do từ chối",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id} sau khi reject, xác nhận status + reason trong response"),
    ]

    f = "Hủy lệnh đang chờ duyệt"
    rows += [
        row("TC-BIZ-012", f, MAKER, "Happy Path", "P1",
            "Maker có 1 lệnh chi chưa ai ký",
            "1. Vào màn trạng thái lệnh đang chờ duyệt\n2. Bấm Hủy lệnh, xác nhận lại\n3. Kiểm tra trạng thái",
            "Lệnh chưa có chữ ký nào",
            "Lệnh chuyển sang trạng thái \"đã hủy\", không checker nào ký được nữa",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/disbursements/{id}/cancel, xác nhận status=cancelled"),
        row("TC-BIZ-013", f, MAKER, "Negative", "P0",
            "Maker có 1 lệnh chi đã đủ chữ ký và đã chuyển tiền xong",
            "1. Thử bấm Hủy lệnh trên 1 lệnh đã hoàn tất\n2. Quan sát phản hồi",
            "Lệnh ở trạng thái completed",
            "Request hủy bị từ chối rõ ràng với thông báo lệnh đã hoàn tất, không thể hủy — không có tiền bị đảo ngược ngoài ý muốn",
            "", "AUTO-API", "curl POST cancel trên lệnh completed, xác nhận response 400/409"),
        row("TC-BIZ-014", f, MAKER, "Negative", "P1",
            "Maker có 1 lệnh chi đã hết hạn (expired do E10)",
            "1. Thử bấm Hủy lệnh trên 1 lệnh đã expired\n2. Quan sát phản hồi",
            "Lệnh ở trạng thái expired",
            "Request hủy bị từ chối vì lệnh đã ở trạng thái cuối (expired), không chuyển trạng thái thêm lần nữa",
            "", "AUTO-API", "curl POST cancel trên lệnh expired, xác nhận response lỗi rõ ràng"),
    ]

    f = "Hộp thư chờ duyệt (My Tasks — Checker)"
    rows += [
        row("TC-BIZ-015", f, CHECKER, "Happy Path", "P0",
            "Checker L1 có ít nhất 1 lệnh đúng cấp đang chờ ký",
            "1. Mở hộp thư \"Chờ tôi duyệt\"\n2. Kiểm tra danh sách chỉ hiện lệnh đúng level, còn hạn",
            "Checker L1, 1 lệnh tier 1 đang treo",
            "Hộp thư hiển thị đúng lệnh cần checker L1 này ký, không hiện lệnh của level khác hoặc đã hết hạn",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/my-tasks với token checker L1, đối chiếu danh sách trả về"),
        row("TC-BIZ-016", f, CHECKER, "Negative", "P1",
            "Có 1 lệnh tier 2 cần cả L1 và L2 ký, L1 chưa ký",
            "1. Đăng nhập với tài khoản checker L2\n2. Mở hộp thư \"Chờ tôi duyệt\"",
            "Lệnh tier 2, L1 chưa ký",
            "Lệnh KHÔNG xuất hiện trong hộp thư của checker L2 cho tới khi L1 đã ký xong (đúng lượt)",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/my-tasks với token checker L2 trước khi L1 ký, xác nhận lệnh không có trong danh sách"),
        row("TC-BIZ-017", f, CHECKER, "Negative", "P1",
            "Có 1 lệnh vừa chuyển sang trạng thái expired (E10)",
            "1. Mở hộp thư \"Chờ tôi duyệt\" của checker liên quan sau khi lệnh expired",
            "Lệnh vừa expired",
            "Lệnh không còn hiện trong hộp thư chờ duyệt của checker sau khi đã expired",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/my-tasks sau khi lệnh expired, xác nhận không còn trong danh sách"),
    ]

    f = "Xem chi tiết lệnh + lịch sử chữ ký"
    rows += [
        row("TC-BIZ-018", f, CHECKER, "Happy Path", "P1",
            "Có 1 lệnh tier 2 đã được checker L1 ký, đang chờ L2",
            "1. Checker L2 mở lệnh từ hộp thư\n2. Xem số tiền, người tạo (maker), lý do chi\n3. Xem mục \"Chữ ký\": ai đã ký ở cấp nào, lúc mấy giờ",
            "Lệnh tier 2 đã có 1 chữ ký L1",
            "Màn chi tiết hiển thị đầy đủ và chính xác: số tiền, maker, lý do, và danh sách chữ ký đã có (tên checker L1 + thời điểm ký)",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id} + /approvals, đối chiếu dữ liệu trả về"),
        row("TC-BIZ-019", f, CHECKER, "Negative", "P2",
            "Có 1 lệnh vừa tạo, chưa ai ký",
            "1. Checker mở lệnh từ hộp thư ngay sau khi maker tạo",
            "Lệnh mới tạo, 0 chữ ký",
            "Mục \"Chữ ký\" hiển thị trống rõ ràng (không phải lỗi hoặc màn trắng gây hiểu lầm), checker hiểu đây là người ký đầu tiên",
            "", "PARTIAL", "curl GET /v1/orgs/{id}/disbursements/{id}/approvals ngay sau khi tạo, xác nhận mảng rỗng"),
    ]

    f = "Ký duyệt/từ chối bằng OTP"
    rows += [
        row("TC-BIZ-020", f, CHECKER, "Happy Path", "P0",
            "Checker L1 có 1 lệnh tier 1 (chỉ cần 1 chữ ký) đang chờ ký",
            "1. Bấm \"Duyệt\"\n2. Hệ thống gửi mã OTP về điện thoại checker\n3. Nhập đúng OTP để xác nhận chữ ký",
            "OTP mock hợp lệ",
            "Chữ ký được ghi nhận là chữ ký cuối cùng cần thiết → tiền chuyển ngay, lệnh chuyển sang completed",
            "", "PARTIAL", "curl POST /v1/orgs/{id}/disbursements/{id}/approve với OTP hợp lệ, xác nhận status + số dư"),
        row("TC-BIZ-021", f, CHECKER, "Negative", "P0",
            "Checker đang ở bước xác nhận chữ ký bằng OTP",
            "1. Bấm \"Duyệt\"\n2. Nhập sai mã OTP\n3. Kiểm tra lệnh có được tính là đã ký chưa",
            "OTP sai",
            "Chữ ký KHÔNG được ghi nhận, lệnh vẫn ở trạng thái chờ đúng như trước khi bấm Duyệt",
            "", "AUTO-API", "curl POST approve với OTP sai, xác nhận response 401/422 và trạng thái lệnh không đổi"),
        row("TC-BIZ-022", f, CHECKER, "Edge Case", "P0",
            "Lệnh tier 1 chỉ cần 1 chữ ký, 2 checker cùng cấp cùng có quyền ký",
            "1. Checker A và checker B cùng bấm \"Duyệt\" gần như đồng thời (đã có OTP hợp lệ sẵn)\n2. Kiểm tra kết quả của cả 2 request",
            "2 request approve gửi song song, cùng 1 lệnh tier 1",
            "FOR UPDATE + UNIQUE(approval_id, checker_id) đảm bảo đúng 1 checker thắng và lệnh được finalize đúng 1 lần; checker còn lại nhận phản hồi rõ ràng lệnh đã được xử lý",
            "E08", "AUTO-API", "curl POST approve song song từ 2 checker (script test gửi đồng thời), đếm số finalize thực tế"),
        row("TC-BIZ-023", f, CHECKER, "Edge Case", "P0",
            "Có 1 lệnh chi tổ chức đang chờ duyệt quá 24 giờ",
            "1. Đặt thời gian tạo lệnh lùi về quá 24h (giả lập)\n2. Chờ cron expiry chạy (hoặc trigger thủ công)\n3. Kiểm tra trạng thái lệnh và workflow",
            "Lệnh pending quá 24h",
            "expires_at + cron quét chuyển approval sang expired, workflow instance fail sạch, không có tiền treo lơ lửng",
            "E10", "AUTO-API", "curl UPDATE created_at về quá khứ qua tool test rồi trigger cron/endpoint expiry, xác nhận status=expired"),
    ]

    f = "Kết quả sau khi ký"
    rows += [
        row("TC-BIZ-024", f, CHECKER, "Happy Path", "P0",
            "Checker vừa ký chữ ký cuối cùng cần thiết cho 1 lệnh",
            "1. Ký xong, xem kết quả ngay trên màn hình",
            "Lệnh tier 1, chữ ký duy nhất vừa hoàn tất",
            "Màn hình hiển thị rõ \"Đã duyệt — đang thực hiện chuyển tiền\", không mơ hồ",
            "", "PARTIAL", "curl POST approve, đối chiếu response status với kỳ vọng completed/processing"),
        row("TC-BIZ-025", f, CHECKER, "Negative", "P1",
            "Checker L1 vừa ký 1 lệnh tier 2 (còn cần L2 ký tiếp)",
            "1. Ký xong ở vai trò L1\n2. Xem kết quả trên màn hình",
            "Lệnh tier 2, mới có 1/2 chữ ký",
            "Màn hình hiển thị rõ \"đã ghi nhận, còn chờ 1 người nữa\", không báo nhầm là đã hoàn tất",
            "", "PARTIAL", "curl POST approve L1, đối chiếu response còn thiếu bao nhiêu chữ ký"),
        row("TC-BIZ-026", f, CHECKER, "Edge Case", "P0",
            "Lệnh tier 1 chỉ cần 1 chữ ký, checker B bấm ký trễ hơn checker A vài giây",
            "1. Checker A ký trước và thắng race\n2. Checker B ký sau vài giây, vẫn cùng lệnh đó",
            "2 checker ký lệch nhau vài giây, cùng 1 lệnh tier 1",
            "Checker B nhận được màn hình báo rõ ràng \"lệnh này vừa được xử lý xong bởi người khác\" thay vì một lỗi hệ thống khó hiểu (không phải HTTP 500 chung chung)",
            "E08", "AUTO-API", "curl POST approve của checker A rồi checker B cách nhau vài giây, xác nhận response B là 409 + message rõ ràng"),
    ]

    assert len(rows) == 26, f"sheet02 expected 26 rows, got {len(rows)}"
    return rows


# ---------------------------------------------------------------------------
# Sheet 03 — 03-admin-ops.xlsx (21 cases, GROUP=ADM) — all AUTO-WEB
# ---------------------------------------------------------------------------
def sheet03_rows():
    R = "Ops Admin"
    METHOD_BASE = "Browser tool: preview_start hpay-admin dev server, navigate + computer + read_page/get_page_text, screenshot làm Evidence"
    rows = []

    f = "Dashboard tổng quan"
    rows += [
        row("TC-ADM-001", f, R, "Happy Path", "P1",
            "Admin đã đăng nhập hpay-admin, có dữ liệu demo (seed-demo-data)",
            "1. Đăng nhập, mở trang Dashboard\n2. Xem các chỉ số KPI trong ngày (số giao dịch, KYC chờ duyệt, cảnh báo gian lận)",
            "Tài khoản admin seed, dữ liệu demo đã chạy",
            "Dashboard hiển thị đúng các chỉ số KPI khớp với dữ liệu thật trong hệ thống (đối chiếu qua API)",
            "", "AUTO-WEB", METHOD_BASE + " trên /dashboard"),
        row("TC-ADM-002", f, R, "Negative", "P2",
            "Có ít nhất 1 hạng mục \"cần chú ý ngay\" (vd KYC chờ lâu, fraud chưa xử lý)",
            "1. Mở Dashboard\n2. Bấm vào khu vực \"cần chú ý ngay\"\n3. Kiểm tra điều hướng",
            "1 hồ sơ KYC pending quá hạn seed sẵn",
            "Bấm vào khu vực cần chú ý điều hướng đúng tới hàng chờ liên quan (vd. /kyc), không dẫn sai trang",
            "", "AUTO-WEB", METHOD_BASE + " — click khu vực cảnh báo, xác nhận URL đích"),
    ]

    f = "Hàng chờ duyệt hồ sơ KYC"
    rows += [
        row("TC-ADM-003", f, R, "Happy Path", "P0",
            "Có ít nhất 1 hồ sơ KYC ở hàng chờ (điểm AI không đủ tự động duyệt) — user 0900000003 theo seed",
            "1. Mở /kyc\n2. Chọn 1 hồ sơ, xem ảnh CCCD + selfie + điểm và lý do AI\n3. Bấm Duyệt",
            "User seed 0900000003, ai_score 64",
            "Hồ sơ chuyển sang approved, user được nâng hạn mức giao dịch/ngày theo cấp KYC mới",
            "", "AUTO-WEB", METHOD_BASE + " trên /kyc/{id}, click Duyệt, đọc toast + gọi API xác nhận status"),
        row("TC-ADM-004", f, R, "Negative", "P1",
            "Có 1 hồ sơ KYC ở hàng chờ",
            "1. Mở chi tiết hồ sơ\n2. Bấm Từ chối, nhập lý do",
            "1 hồ sơ pending",
            "Hồ sơ chuyển sang rejected, user nhận được thông báo và có thể nộp lại hồ sơ mới",
            "", "AUTO-WEB", METHOD_BASE + " trên /kyc/{id}, click Từ chối, xác nhận trạng thái"),
        row("TC-ADM-005", f, R, "Negative", "P2",
            "Có ≥2 hồ sơ KYC ở hàng chờ",
            "1. Dùng phím tắt bàn phím để duyệt nhanh hồ sơ đang chọn\n2. Kiểm tra hồ sơ tiếp theo được focus tự động",
            "≥2 hồ sơ pending",
            "Phím tắt duyệt đúng hồ sơ đang chọn, con trỏ tự chuyển sang hồ sơ kế tiếp trong hàng chờ",
            "", "AUTO-WEB", METHOD_BASE + " — dùng computer key để gửi phím tắt, đọc DOM xác nhận focus"),
    ]

    f = "Giám sát gian lận"
    rows += [
        row("TC-ADM-006", f, R, "Happy Path", "P0",
            "Có ≥1 giao dịch bị gắn cờ nghi ngờ gian lận (fraud flag) trong seed",
            "1. Mở /fraud\n2. Xem lý do bị gắn cờ (luật cứng + giải thích AI)\n3. Bấm \"Xác nhận gian lận\"",
            "1 fraud flag seed sẵn (user 0900000004)",
            "Giao dịch/flag chuyển sang trạng thái confirmed_fraud, tài khoản liên quan bị khóa hoặc đưa vào quy trình reversal",
            "", "AUTO-WEB", METHOD_BASE + " trên /fraud/{id}, click Xác nhận, xác nhận trạng thái qua API"),
        row("TC-ADM-007", f, R, "Negative", "P1",
            "Có 1 fraud flag ở hàng chờ",
            "1. Mở chi tiết flag\n2. Bấm \"Bỏ qua cảnh báo\" (false positive)",
            "1 fraud flag pending",
            "Flag chuyển sang dismissed, giao dịch/tài khoản liên quan không bị khóa",
            "", "AUTO-WEB", METHOD_BASE + " trên /fraud/{id}, click Bỏ qua, xác nhận trạng thái"),
        row("TC-ADM-008", f, R, "Negative", "P2",
            "Có 1 fraud flag đã quá hạn xử lý",
            "1. Mở danh sách /fraud, lọc theo \"quá hạn\"\n2. Kiểm tra hiển thị cảnh báo trực quan cho flag quá hạn",
            "1 fraud flag có deadline đã qua",
            "Flag quá hạn được đánh dấu nổi bật trực quan (màu/badge) khác với flag còn trong hạn",
            "", "AUTO-WEB", METHOD_BASE + " trên /fraud với filter quá hạn, đọc DOM xác nhận badge"),
    ]

    f = "Giám sát luồng xử lý (Workflow Monitor)"
    rows += [
        row("TC-ADM-009", f, R, "Happy Path", "P1",
            "Có nhiều workflow instance ở các trạng thái khác nhau (đang chạy/lỗi/kẹt)",
            "1. Mở /workflows\n2. Lọc theo trạng thái/loại giao dịch\n3. Bấm vào 1 instance xem chi tiết",
            "Dữ liệu workflow đa dạng từ seed-demo-data",
            "Danh sách lọc đúng theo tiêu chí; màn chi tiết hiển thị đúng bước đang dừng và lý do",
            "", "AUTO-WEB", METHOD_BASE + " trên /workflows, apply filter, click 1 row, đọc chi tiết step"),
        row("TC-ADM-010", f, R, "Edge Case", "P1",
            "Có 1 workflow instance bị kẹt do worker chết giữa chừng (DEBIT xong, chưa CREDIT — E04/E05)",
            "1. Mở /workflows/{id} của instance kẹt\n2. Kiểm tra bước đang dừng lại được hiển thị đúng, không side-effect đôi",
            "1 instance kẹt mô phỏng E04/E05",
            "Chi tiết instance hiển thị đúng bước đang treo; sau khi hệ thống recover, không có DEBIT/CREDIT bị lặp (đối chiếu ledger qua API)",
            "E04", "AUTO-WEB", METHOD_BASE + " trên /workflows/{id} + curl GET ledger để xác nhận không double side-effect"),
        row("TC-ADM-011", f, R, "Edge Case", "P1",
            "Có 1 workflow instance kẹt quá lâu (updated_at lùi quá ngưỡng)",
            "1. Trigger sweeper (recovery) qua endpoint/CI script\n2. Mở /workflows/{id} kiểm tra trạng thái cập nhật sau khi sweeper chạy",
            "1 instance kẹt, sweeper chạy 2 lần song song (mô phỏng E20)",
            "Instance chỉ được claim và re-emit đúng 1 lần dù có 2 sweeper chạy song song; màn Workflow Monitor phản ánh đúng trạng thái cuối cùng, không lặp bước",
            "E20", "AUTO-WEB", METHOD_BASE + " trên /workflows/{id} sau khi trigger sweeper song song qua script test"),
    ]

    f = "Đối soát giao dịch hằng ngày"
    rows += [
        row("TC-ADM-012", f, R, "Happy Path", "P1",
            "Có dữ liệu giao dịch + sao kê ngân hàng mock đã seed cho 1 ngày",
            "1. Mở /recon, bấm \"Chạy đối soát\"\n2. Xem danh sách khớp/lệch/thiếu\n3. Bấm Xuất Excel",
            "Dữ liệu 1 ngày đối soát demo",
            "Kết quả đối soát hiển thị đúng 3 nhóm (khớp/lệch/thiếu); file Excel xuất ra tải về thành công và khớp với dữ liệu trên màn hình",
            "", "AUTO-WEB", METHOD_BASE + " trên /recon, click Chạy + Xuất Excel, xác nhận link tải xuất hiện"),
        row("TC-ADM-013", f, R, "Negative", "P2",
            "Có kết quả đối soát với ít nhất 1 mục lệch",
            "1. Mở 1 mục lệch trong kết quả đối soát\n2. Đọc tóm tắt nguyên nhân lệch do AI viết",
            "1 mục lệch trong kết quả recon",
            "Tóm tắt AI hiển thị bằng ngôn ngữ tự nhiên dễ hiểu, giải thích đúng hướng nguyên nhân lệch (thiếu ở phía nào)",
            "", "AUTO-WEB", METHOD_BASE + " trên /recon, mở chi tiết mục lệch, đọc nội dung AI summary"),
        row("TC-ADM-014", f, R, "Edge Case", "P1",
            "Có giao dịch xảy ra ngay cắt ngang mốc nửa đêm (00:00) của ngày đối soát",
            "1. Chạy đối soát cho ngày có giao dịch cắt ngang nửa đêm\n2. Kiểm tra giao dịch này được phân loại đúng, không báo lệch giả",
            "1 giao dịch có timestamp gần 00:00",
            "Recon so khớp theo ref_code chứ không chỉ theo ngày lịch — giao dịch cắt cutoff được phân loại riêng rõ ràng, không rơi vào nhóm \"lệch\" do clock skew",
            "E16", "AUTO-WEB", METHOD_BASE + " trên /recon với dữ liệu giao dịch cutoff nửa đêm, kiểm tra phân loại kết quả"),
    ]

    f = "My Tasks của Admin"
    rows += [
        row("TC-ADM-015", f, R, "Happy Path", "P0",
            "Admin A đã đề xuất 1 thao tác nhạy cảm (hoàn tiền) đang chờ Admin B duyệt",
            "1. Đăng nhập với Admin B, mở /tasks\n2. Xem lý do đề xuất của Admin A\n3. Bấm Duyệt",
            "1 reversal request pending từ Admin A",
            "Thao tác (hoàn tiền) được thực thi thật sau khi Admin B duyệt; My Tasks cập nhật trạng thái ngay tại chỗ",
            "", "AUTO-WEB", METHOD_BASE + " trên /tasks, click Duyệt, xác nhận qua API ledger/reversal"),
        row("TC-ADM-016", f, R, "Negative", "P1",
            "Admin A đã đề xuất 1 thao tác nhạy cảm đang chờ Admin B duyệt",
            "1. Đăng nhập với Admin B\n2. Bấm Từ chối kèm lý do",
            "1 task pending",
            "Thao tác dừng lại, không được thi hành; task chuyển sang trạng thái rejected kèm lý do",
            "", "AUTO-WEB", METHOD_BASE + " trên /tasks, click Từ chối, xác nhận trạng thái"),
        row("TC-ADM-017", f, R, "Edge Case", "P0",
            "Có 1 task trong My Tasks mà 2 admin (2 browser context) đều có quyền xử lý",
            "1. Mở 2 browser context (2 phiên đăng nhập admin khác nhau) cùng nhìn 1 task\n2. Cả 2 bấm \"Duyệt\" gần như đồng thời",
            "1 task, 2 context cùng thao tác song song",
            "1 context nhận 200 + row cập nhật đúng; context còn lại nhận lỗi 409 ERR_ALREADY_FINALIZED kèm toast rõ ràng, row tự cập nhật tại chỗ, không phải reload cả bảng",
            "A01", "AUTO-WEB", "Browser tool: 2 tab riêng biệt (2 tabs_create) đăng nhập 2 admin, click Duyệt gần như đồng thời, đọc network response + DOM"),
    ]

    f = "Tìm kiếm nhật ký audit"
    rows += [
        row("TC-ADM-018", f, R, "Happy Path", "P1",
            "Có nhiều bản ghi audit_logs từ các thao tác trước đó (KYC approve, policy change...)",
            "1. Mở /audit\n2. Gõ từ khoá tìm kiếm (vd tên admin hoặc loại thao tác)\n3. Xem kết quả",
            "Từ khoá khớp với ≥1 bản ghi audit có sẵn",
            "Kết quả tìm kiếm hiển thị đúng: ai, làm gì, khi nào, vì sao — khớp với dữ liệu audit_logs thật",
            "", "AUTO-WEB", METHOD_BASE + " trên /audit, nhập từ khoá search, đối chiếu kết quả với API"),
        row("TC-ADM-019", f, R, "Negative", "P2",
            "Có ≥1 bản ghi audit log",
            "1. Thử tìm cách sửa/xoá 1 bản ghi audit log (không có action nào trên UI cho việc này)\n2. Xác nhận qua API cũng không có endpoint update/delete",
            "1 bản ghi audit log cụ thể",
            "Không có action sửa/xoá nào khả dụng trên UI; gọi thẳng API update/delete (nếu thử) bị từ chối — audit log bất biến (immutable)",
            "", "AUTO-WEB", METHOD_BASE + " trên /audit — xác nhận UI không có nút sửa/xoá + curl thử PUT/DELETE endpoint audit"),
    ]

    f = "Quản lý tổ chức &amp; chính sách"
    rows += [
        row("TC-ADM-020", f, R, "Happy Path", "P1",
            "Có ≥1 tổ chức doanh nghiệp hoạt động trên hệ thống (seed)",
            "1. Mở /orgs\n2. Chọn 1 tổ chức\n3. Xem thành viên + vai trò + chính sách duyệt chi hiện tại",
            "1 tổ chức seed với thành viên + policy",
            "Trang chi tiết hiển thị đúng danh sách thành viên, vai trò của từng người, và chính sách duyệt chi hiện hành",
            "", "AUTO-WEB", METHOD_BASE + " trên /orgs/{id}, đối chiếu dữ liệu hiển thị với API"),
        row("TC-ADM-021", f, R, "Negative", "P2",
            "Admin đang xem chi tiết 1 tổ chức, cần hỗ trợ điều chỉnh policy hộ khách hàng",
            "1. Mở màn sửa policy của tổ chức\n2. Điều chỉnh 1 tier, lưu lại\n3. Kiểm tra thay đổi có hiệu lực",
            "1 tổ chức, sửa ngưỡng tier",
            "Policy được cập nhật thành version mới, hiển thị đúng trên UI và qua API, không ảnh hưởng lệnh đang treo (theo đúng E09)",
            "", "AUTO-WEB", METHOD_BASE + " trên /orgs/{id}, sửa policy, xác nhận version mới qua API"),
    ]

    assert len(rows) == 21, f"sheet03 expected 21 rows, got {len(rows)}"
    return rows


# ---------------------------------------------------------------------------
# Sheet 04 — 04-platform-crosscutting.xlsx (10 cases, GROUP=PLT) — all AUTO-API
# ---------------------------------------------------------------------------
def sheet04_rows():
    R = "System"
    rows = []

    f = "Phát hiện gian lận tự động"
    rows += [
        row("TC-PLT-001", f, R, "Happy Path", "P1",
            "hpay-backend đang chạy, có user với ví hoạt động",
            "1. Thực hiện 1 giao dịch bình thường (số tiền hợp lý, thiết bị quen thuộc)\n2. Kiểm tra giao dịch có bị chặn không",
            "Giao dịch 100,000 VND, thiết bị/tần suất bình thường",
            "Luật cứng chấm điểm rủi ro thấp → giao dịch được cho qua ngay, không bị AI can thiệp, không delay bất thường",
            "", "AUTO-API", "curl POST /v1/transactions/transfer, đo latency + xác nhận không có fraud flag phát sinh"),
        row("TC-PLT-002", f, R, "Negative", "P1",
            "hpay-backend đang chạy",
            "1. Tạo giao dịch với đặc điểm rơi vào \"vùng xám\" (vd tần suất hơi cao nhưng chưa chắc chắn)\n2. Kiểm tra hệ thống có gọi AI giải thích thêm không",
            "Giao dịch thiết kế để rơi đúng ngưỡng vùng xám của rule engine",
            "Hệ thống gọi AI để giải thích thêm lý do nghi ngờ; AI KHÔNG tự ý chặn tiền — giao dịch vẫn được ghi nhận, chờ luật cứng/người quyết định cuối",
            "", "AUTO-API", "curl POST transfer với đặc điểm vùng xám, kiểm tra fraud_flags có bản ghi kèm ai_explanation, transaction chưa bị block"),
        row("TC-PLT-003", f, R, "Negative", "P0",
            "hpay-backend đang chạy",
            "1. Tạo giao dịch với đặc điểm rõ ràng nguy hiểm (vượt xa ngưỡng luật cứng)\n2. Kiểm tra giao dịch có bị chặn ngay không",
            "Giao dịch vượt xa ngưỡng tốc độ/số tiền bất thường của rule engine",
            "Luật cứng chặn ngay lập tức, không chờ AI; quyết định chặn là do luật cứng, không phải do AI tự quyết",
            "", "AUTO-API", "curl POST transfer với đặc điểm rõ ràng nguy hiểm, xác nhận response bị chặn (403/422) ngay lập tức"),
    ]

    f = "An toàn khi bấm lại (Idempotency)"
    rows += [
        row("TC-PLT-004", f, R, "Happy Path", "P0",
            "hpay-backend đang chạy, user có ví hoạt động",
            "1. Gửi POST /v1/transactions/transfer với Idempotency-Key X (lần 1)\n2. Gửi lại đúng request với cùng Idempotency-Key X (lần 2, giả lập bấm lại)\n3. So sánh 2 response",
            "Idempotency-Key = X (UUID cố định), cùng payload",
            "Cả 2 response trả về cùng refCode/transaction ID; ví chỉ bị trừ/cộng tiền đúng 1 lần duy nhất",
            "E01", "AUTO-API", "curl POST transfer 2 lần với cùng Idempotency-Key header, so response + số dư trước/sau"),
        row("TC-PLT-005", f, R, "Edge Case", "P0",
            "Có 1 workflow instance đã publish event kết quả 1 lần",
            "1. Giả lập outbox relay publish lại đúng event đó lần 2 (retry)\n2. Kiểm tra side-effect (vd cộng tiền, gửi thông báo) có bị lặp không",
            "1 event kết quả, publish 2 lần liên tiếp",
            "Consumer check step_executions (instance, step, attempt) trước khi side-effect — event thứ 2 bị bỏ qua, không có side-effect kép",
            "E06", "AUTO-API", "curl/script trigger lại đúng event qua outbox relay test hook, đếm số side-effect thực tế (vd số dòng ledger)"),
        row("TC-PLT-006", f, R, "Negative", "P1",
            "hpay-backend đang chạy, có endpoint topup và transfer đều nhận Idempotency-Key",
            "1. Gửi request topup với key Y, lặp lại 2 lần\n2. Gửi request transfer với key Z (khác Y), lặp lại 2 lần\n3. Kiểm tra cả 2 luồng đều dedupe đúng theo đúng khoá riêng của chính nó",
            "Idempotency-Key Y (topup) và Z (transfer) độc lập nhau",
            "Mỗi luồng chỉ tạo đúng 1 bản ghi giao dịch cho khoá tương ứng của nó, không lẫn lộn giữa 2 loại request khác nhau dùng chung cơ chế idempotency",
            "E01", "AUTO-API", "curl POST topup x2 (key Y) + POST transfer x2 (key Z), xác nhận mỗi loại chỉ tạo đúng 1 giao dịch"),
    ]

    f = "Tự phục hồi khi có sự cố"
    rows += [
        row("TC-PLT-007", f, R, "Happy Path", "P0",
            "Có 1 giao dịch dừng lại giữa chừng do sự cố mô phỏng (mất mạng/server restart)",
            "1. Tạo giao dịch rồi làm nó \"kẹt\" giữa chừng (mô phỏng worker chết)\n2. Chờ tiến trình quét định kỳ (recovery sweeper) chạy\n3. Kiểm tra giao dịch có được xử lý tiếp không",
            "1 workflow instance bị kẹt nhân tạo",
            "Giao dịch được thử lại đúng từ bước đang dở dang (không lặp lại các bước đã xong), cuối cùng hoàn tất hoặc thất bại rõ ràng có hoàn tiền nếu cần",
            "", "AUTO-API", "curl trigger sweeper endpoint/script + GET trạng thái workflow instance trước/sau"),
        row("TC-PLT-008", f, R, "Edge Case", "P0",
            "Redis đang chạy bình thường, có thể tắt tạm thời trong môi trường test",
            "1. Tắt Redis hoàn toàn\n2. Gửi request tạo giao dịch mới\n3. Kiểm tra response và trạng thái DB",
            "Redis down, 1 request tạo transaction mới",
            "API vẫn trả response thành công (202), transaction được ghi vào DB + outbox; engine dừng tiến độ xử lý tiếp (chờ Redis) nhưng KHÔNG mất dữ liệu, không lỗi 500",
            "E17", "AUTO-API", "docker stop redis container rồi curl POST transfer, kiểm tra response + bản ghi DB, sau đó bật lại Redis xác nhận engine tiếp tục"),
        row("TC-PLT-009", f, R, "Edge Case", "P1",
            "Có 1 giao dịch vừa commit DB thành công nhưng publish event thất bại (broker outage mô phỏng)",
            "1. Giả lập broker (Redis Streams) outage ngay sau khi DB commit\n2. Khôi phục broker\n3. Kiểm tra outbox relay có publish lại event không",
            "1 giao dịch, broker outage ngắn hạn ngay sau commit",
            "Event nằm sẵn trong bảng outbox (transactional outbox) trong lúc broker down; sau khi broker hồi phục, outbox relay tự động publish lại, không mất event",
            "E05", "AUTO-API", "docker stop redis tạm thời đúng lúc transaction commit, bật lại, kiểm tra outbox.published_at chuyển từ NULL sang có giá trị"),
        row("TC-PLT-010", f, R, "Edge Case", "P1",
            "Có 1 workflow instance bị đánh dấu \"kẹt\" (updated_at lùi quá ngưỡng)",
            "1. Chạy đồng thời 2 tiến trình sweeper trên cùng 1 instance kẹt\n2. Đếm số lần claim thành công và số lần re-emit sự kiện",
            "1 instance kẹt, 2 sweeper chạy song song (goroutine test)",
            "Lease claim bằng UPDATE ... RETURNING đảm bảo chỉ 1 sweeper claim thành công; chỉ đúng 1 lần re-emit sự kiện, instance không bị recover trùng",
            "E20", "AUTO-API", "script test chạy 2 goroutine gọi SweepStuck() song song, query outbox đếm số event chưa publish còn lại"),
    ]

    assert len(rows) == 10, f"sheet04 expected 10 rows, got {len(rows)}"
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
            method = f"curl/PowerShell trực tiếp vào hpay-backend API tương ứng, đối chiếu với {test_name} — so response/DB state với Expected Result"
            pre = f"hpay-backend chạy với DB+Redis sạch (hoặc trạng thái mô phỏng cần thiết theo kịch bản); tương ứng test tự động {test_name} trong src/backend/{'test/integration' if 'integration' in test_name.lower() or True else ''}"
        elif prefix == "M":
            role, executor = "Personal User / Business Checker", "HUMAN"
            method = "Cần thiết bị thật/simulator Expo — Claude không có công cụ điều khiển màn hình mobile; người kiểm thử làm theo đúng kịch bản và đối chiếu với unit test JS tương ứng"
            pre = f"hpay-mobile chạy trên thiết bị thật hoặc simulator, tài khoản demo hợp lệ; tương ứng {test_name} trong src/mobile/src/**/__tests__"
        else:  # A
            role, executor = "Ops Admin", "AUTO-WEB"
            method = f"Browser tool: preview_start hpay-admin dev server, 2 tab/context khi cần dàn dựng race, computer/read_page/network xác nhận hành vi — tương ứng {test_name}"
            pre = "hpay-admin dev server chạy, ≥2 tài khoản admin fixture (maker/checker hoặc 2 admin cùng quyền) khi kịch bản cần 2 actor đồng thời"

        steps = (
            f"1. Dàn dựng đúng điều kiện của {eid}: {desc}\n"
            f"2. Thực hiện hành động kích hoạt kịch bản (transaction/API call/thao tác UI tương ứng)\n"
            f"3. Quan sát/đo kết quả thực tế (response, DB state, UI state) và so với Expected Result\n"
            f"4. Đối chiếu chéo với test tự động {test_name} (nếu case này đã có coverage tự động) để xác nhận hành vi nhất quán"
        )
        rows.append(row(
            f"TC-EDG-{eid}", f"Edge Case {eid} ({repo})", role, "Edge Case",
            "P0" if prefix in ("E", "M") else "P1",
            pre, steps, f"Xem mô tả kịch bản {eid}: {desc}", expected,
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
    "Bộ checklist QA thủ công song song với bộ test tự động (TestEdge_*) đã có trong "
    "hpay-backend / hpay-mobile / hpay-admin. Không thay thế, không trùng lặp phạm vi — "
    "xem docs/qa/plan/implementation-plan.html và solution-architecture.html để biết "
    "toàn bộ bối cảnh và quyết định thiết kế."
)

SCHEMA_ROWS = [
    ("A", "Test ID", "TC-<GROUP>-<NNN> hoặc TC-EDG-<E/M/A-ID>",
     "GROUP ∈ {PER, BIZ, ADM, PLT} cho sheet 01-04 (đánh số tuần tự trong nhóm); sheet 05 dùng chính edge-case ID gốc"),
    ("B", "Feature", "Text — tên chức năng", "Sheet 01-04: đúng tên trong FEATURES[].title của functional-overview.vi.html. Sheet 05: \"Edge Case <ID> (<repo>)\""),
    ("C", "Role / Actor", "Enum", "Personal User · Business Maker · Business Checker · Org Admin · Ops Admin · System"),
    ("D", "Test Type", "Enum", "Happy Path · Negative · Edge Case · Boundary · Security/AuthZ"),
    ("E", "Priority", "Enum", "P0 (chặn release, mặc định cho mọi thứ đụng tới tiền) · P1 · P2"),
    ("F", "Precondition / Setup", "Text nhiều dòng", "Dữ liệu seed cần có trước khi chạy case, để case chạy lại được (repeatable)"),
    ("G", "Test Steps", "Text đánh số 1. 2. 3.", "Đủ chi tiết để người lạ (hoặc Claude) làm theo không cần hỏi lại"),
    ("H", "Test Data", "Text/JSON ngắn", "Số điện thoại demo, số tiền cụ thể, OTP mock... tách riêng khỏi Steps"),
    ("I", "Expected Result", "Text", "Mô tả rõ trạng thái cuối + response code/UI state kỳ vọng, không mơ hồ"),
    ("J", "Related Edge ID", "E01-E20 / M01-M12 / A01-A07 hoặc trống", "Cột traceability bắt buộc — cầu nối case chức năng và case edge"),
    ("K", "Executor", "Enum", "AUTO-API · AUTO-WEB · PARTIAL · HUMAN — xem bảng Legend bên dưới"),
    ("L", "Execution Method", "Text ngắn", "Cụ thể hoá cột K, vd \"curl POST /v1/transactions/transfer\""),
    ("M", "Status", "Enum", "Not Run · Pass · Fail · Blocked · N/A — mặc định Not Run khi sinh file"),
    ("N", "Actual Result", "Text", "Điền khi thực thi — AUTO-API/AUTO-WEB: Claude điền response thật"),
    ("O", "Evidence", "Text (đường dẫn/link)", "Screenshot, response JSON log, hoặc tham chiếu chéo TestEdge_X trong CI"),
    ("P", "Executed By", "Text", "\"Claude (auto)\" hoặc tên người"),
    ("Q", "Last Run Date", "Date (yyyy-mm-dd)", "ISO format"),
    ("R", "Defect ID / Notes", "Text", "Link Jira/GitHub issue nếu Fail, hoặc ghi chú tự do"),
]

LEGEND_ROWS = [
    ("AUTO-API", "Claude tự chạy toàn bộ bằng HTTP request trực tiếp (curl/PowerShell) vào backend, đọc response, tự điền Pass/Fail — không cần người",
     "Hầu hết sheet 04 (platform), phần lớn E-series trong sheet 05, và phần \"hậu trường\" (state/response) dù UI là mobile"),
    ("AUTO-WEB", "Claude tự chạy toàn bộ qua Browser automation tool trên hpay-admin — click, đọc DOM, chụp screenshot làm Evidence",
     "Toàn bộ sheet 03 (admin ops), A-series trong sheet 05"),
    ("PARTIAL", "Claude xác nhận được đúng hành vi backend qua API (tiền có chuyển đúng không, state máy đúng không, response code đúng không) nhưng KHÔNG xác nhận được UI mobile hiển thị đúng — cần người xem lại phần UI để case được tính \"Pass\" đầy đủ. QUY TẮC CỨNG: không bao giờ tự chuyển Pass, chỉ chuyển Blocked chờ người xác nhận UI.",
     "Đa số sheet 01-02 (personal + business chạy trên mobile app) — Claude điền sẵn phần backend, để trống ô UI cho người ký xác nhận"),
    ("HUMAN", "Cần thiết bị thật/simulator, phần cứng (camera thật cho KYC, cảm biến vân tay/Face ID), hoặc quan sát trực quan không đo được qua API — Claude không chạm được phần nào của case này",
     "M-series (biometric, camera KYC, background/foreground), phần capture ảnh KYC trong sheet 01, mọi assertion về \"app hiển thị đúng\" không quan sát được qua API"),
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
    c = ws.cell(row=r, column=1, value=f"{case_count} test case · Sheet chính \"Test Cases\" · Nguồn: docs/qa/plan/implementation-plan.html §4/§6/§8")
    c.font = Font(italic=True, size=10, color="6B7280")
    c.alignment = Alignment(wrap_text=True)
    r += 2

    ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=4)
    intro_cell = ws.cell(row=r, column=1, value=README_INTRO)
    intro_cell.font = body_font
    intro_cell.alignment = Alignment(wrap_text=True, vertical="top")
    ws.row_dimensions[r].height = 45
    r += 2

    ws.cell(row=r, column=1, value="Schema cột (A–R)").font = h2_font
    r += 1
    headers2 = ["Cột", "Tên cột", "Kiểu / Giá trị hợp lệ", "Ghi chú"]
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

    ws.cell(row=r, column=1, value="Legend — Executor (cột K)").font = h2_font
    r += 1
    headers3 = ["Nhãn", "Ý nghĩa", "Áp dụng cho", ""]
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
        "TC-<GROUP>-<NNN>  — sheet 01-04, đánh số tuần tự trong nhóm. GROUP ∈ {PER, BIZ, ADM, PLT}\n"
        "TC-EDG-<EDGE-ID>  — sheet 05, ID = chính edge-case ID gốc (E01-E20 backend, M01-M12 mobile, A01-A07 admin)\n\n"
        "Traceability 2 chiều: case chức năng chạm rủi ro đã có edge-ID ghi đúng ID đó vào cột J (không tạo TC-EDG mới trùng); "
        "mọi 1/37 ID gốc đều có đúng 1 dòng \"chốt\" trong 05-edge-cases.xlsx."
    ))
    ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=4)
    conv.alignment = Alignment(wrap_text=True, vertical="top")
    conv.font = body_font
    ws.row_dimensions[r].height = 90
    r += 2

    note = ws.cell(row=r, column=1, value=(
        "Quy tắc cứng: case Executor=PARTIAL không bao giờ được tự động đánh dấu Pass — chỉ Blocked (chờ người xác nhận UI mobile) "
        "hoặc Not Run. Case Executor=HUMAN không bị Claude chạm vào — giữ nguyên Not Run, lý do ghi ở cột R."
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


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    build_workbook("01-personal-user.xlsx", sheet01_rows(), os.path.join(OUT_DIR, "01-personal-user.xlsx"))
    build_workbook("02-business-maker-checker.xlsx", sheet02_rows(), os.path.join(OUT_DIR, "02-business-maker-checker.xlsx"))
    build_workbook("03-admin-ops.xlsx", sheet03_rows(), os.path.join(OUT_DIR, "03-admin-ops.xlsx"))
    build_workbook("04-platform-crosscutting.xlsx", sheet04_rows(), os.path.join(OUT_DIR, "04-platform-crosscutting.xlsx"))
    build_workbook("05-edge-cases.xlsx", sheet05_rows(), os.path.join(OUT_DIR, "05-edge-cases.xlsx"))
    print("Done. 5 files written to", OUT_DIR)


if __name__ == "__main__":
    main()
