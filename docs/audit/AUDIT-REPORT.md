# HPay – Báo cáo rà soát toàn dự án

Ngày: 2026-11-21. Phạm vi: `hpay-backend`, `hpay-admin`, `hpay-mobile`, `hpay-docs`, `ref-docs`.
Phương pháp: 4 agent đọc code song song, chỉ đọc, không sửa file nào.

> **Giới hạn.** Báo cáo dựa trên đọc code và grep, chưa chạy test hay chạy ứng dụng. Mục nào ghi **[chưa xác minh]** là nghi vấn cần đọc thêm. Chưa kiểm tra:
> - `workflow/engine.go` (mới grep), `gateway.go`, `round_up.go`, `fraud/rules.go`.
> - Đối chiếu từng endpoint và field giữa `openapi.yaml` và frontend.
> - Hai file docs lớn nhất (`00-backend-design-v2-base.html`, `technical-overview.*`), các file xlsx và `ref-docs/*.docx`.
> - Số dòng trong phần docs có thể chưa chính xác vì kết quả grep bị cắt.

---

## Phụ lục xác minh (2026-11-21, đã đọc lại code)

Kết quả kiểm chứng các mục **[chưa xác minh]** bên dưới. Phần này **ghi đè** mọi chỗ mâu thuẫn trong báo cáo.

| Mục | Kết luận | Bằng chứng |
|---|---|---|
| MOVE_FUNDS double-post (BE-C1) | **CONFIRMED**, đã tìm guard bác bỏ nhưng không thấy | `handlers.go` ~82-122: tx riêng `Begin→ledger.Post→Commit`; `consumer.go:164-212 recordAndEmit` là tx khác. `ledger_entries` chỉ có `idx_ledger_txn` không UNIQUE (000002:83). `PostWithQueries` không kiểm tra txn đã có entry |
| Compensation fail | **PARTIAL** | `engine.go:301-310`: chỉ log Error rồi set `failed`. Enum `wf_status` không có `compensation_failed`. Nhánh này không gọi `FailTransactionGuarded`, nên transaction có thể kẹt `processing`. `ev.Retryable` bị bỏ qua. `ClaimStuckInstances` chỉ claim `running`, nên instance kẹt `compensating` không được recovery |
| `dispatchCompensate` Attempt=101 | **CONFIRMED** (không retry được) | `engine.go:337`; `consumer.go:111-114` thấy bản ghi cũ thì chỉ re-emit kết quả cũ |
| SSRF gateway | **CONFIRMED** | `action_pipeline.go:81-107`: URL lấy từ `srv.Actions` trong DB, chỉ check prefix `http`; `gateway.go:59` gọi thẳng. Không allowlist, không chặn IP private/metadata, không giới hạn redirect. Có timeout 10s, body tối đa 64KB. HTTP thật chỉ bật khi `Env=="production"` |
| Self-approval org | **CONFIRMED** | `org_members` có `UNIQUE(org_id,user_id)` (000005:36). `eligibility()` (`approval/service.go:221-250`) không có `checkerID != CreatedBy`. Chỉ luồng admin có (`admin.go:67`) |
| Idempotency middleware | **PARTIAL** | `RequireIdempotencyKey` chỉ gắn ở `router.go:118` và `:156`. **Không** gắn cho `/money-requests`, `/money-requests/split`, `POST /vaults`, `PATCH /vaults/:id/round-up-target` |
| `WalletHandler.Topup`, `TransferToUser` | **Dead code** (không mount) | Hạ BE-M5 (legacy bỏ qua offerguard) xuống Low vì không reachable |
| asynq / golang-migrate / testcontainers | **Docs sai** | `go.mod` chỉ có gin, jwt, uuid, pgx, godotenv, go-redis, excelize, x/crypto. Cron/recovery thật là goroutine trong process (`main.go:146-151`). Nhiều replica sẽ chạy trùng (recon cron chưa kiểm lease). Migration chạy bằng CLI `migrate` ngoài (Makefile, CI), không chạy trong server hay Dockerfile |
| Config prod guard | **CONFIRMED** (yếu) | `config.go:72` `APP_ENV` default `dev`, không validate. Code so sánh `"production"` (`main.go:117`, `seed/main.go:43`) nhưng giá trị đó không nằm trong danh sách hợp lệ (dev\|ci\|demo) |
| Mojibake | **REFUTED** | `transfer/service.go` và `router.tsx` là UTF-8 hợp lệ, không BOM, 0 U+FFFD. Chỉ là console hiển thị sai codepage. **Bỏ mục mojibake khỏi danh sách lỗi.** Tuy vậy lỗi `errors.New("không tìm thấy giao dịch")` vẫn thiếu trong `errTable` |
| OpenAPI vs frontend | **PARTIAL**, lệch thật (chi tiết bên dưới) | `api/openapi.yaml` có 63 path |

**Endpoint frontend gọi nhưng backend/openapi không có:**
- Mobile: `/v1/auth/verify-otp`, `/v1/engine/status`, `/v1/money-requests/{id}/pay`, `/v1/wallets/topup`, `/v1/transactions/transfer`.
- Admin: `/v1/admin/fraud/flags` (+`/{id}/resolve`) trong khi backend là `/fraud-flags`; `/v1/admin/kyc/{id}/decision` (backend là `/kyc/:id/review`); `/v1/admin/services/{id}/status` (backend `PUT /services/:id`); `POST /v1/admin/services`; `POST/PUT /v1/admin/offers/{id}` (backend chỉ có GET); `/v1/p2p-notify`.
- Enum workflow instance status trong openapi (~dòng 1726) thiếu `waiting_approval`, `compensating`, `recovering`. Mobile dùng string cho amount ở `useFeePreview` nhưng type number ở response.
- **Chưa làm:** so enum status của transaction/money-request/KYC với DB, và so từng field response.

---

## 0. Việc cần làm ngay (top ưu tiên)

| # | Việc | Mức |
|---|------|-----|
| 1 | Xoay (rotate) toàn bộ secret trong `hpay-backend/.env`: mật khẩu DB Neon, Cloudinary API secret, JWT secret. Nội dung đã xuất hiện trong output của lần audit này. | Critical |
| 2 | MOVE_FUNDS không idempotent: redelivery hoặc crash giữa 2 tx có thể trừ tiền 2 lần (BE-C1) | Critical |
| 3 | `/transactions/confirm` và `/verify` không kiểm tra PIN/OTP và có IDOR; request đã chuyển tiền ngay (BE-C2) | Critical |
| 4 | Idempotency-Key là UNIQUE toàn cục, không theo user, không so khớp payload (BE-H1) | High |
| 5 | Mobile hard-code PIN `123456` cho mọi giao dịch và cho khóa app | Critical |
| 6 | Admin không phân quyền theo role (cả backend lẫn route frontend) | High |
| 7 | `APP_ENV` mặc định `dev`: lộ `otp_dev`, secret yếu được chấp nhận. Mock-bank callback dùng secret mặc định, route vẫn mount ở mọi env | High |
| 8 | Thiếu constraint DB: unique reversal, ledger idempotency, unique theo user cho idempotency key (gom vào migration 000014) | High |

---

## 1. Lỗ hổng logic và bảo mật – Backend

### Critical

**BE-C1. MOVE_FUNDS có thể trừ tiền 2 lần** (`internal/workflow/handlers/handlers.go:82-122`, `workflow/consumer.go:102-161`, migration 000002:70-84)
- `Wallet.Execute` commit `ledger.Post` trong một tx, còn `step_executions` được ghi ở tx khác (`recordAndEmit`).
- Nếu crash giữa hai tx, hoặc outbox redeliver (at-least-once), step chạy lại và post ledger lần hai.
- `ledger.Post` không kiểm tra `txnID` đã post chưa. `ledger_entries` không có UNIQUE chặn việc này. Compensate bị cùng lỗi.
- Fix: ghi step claim và ledger trong cùng tx, hoặc INSERT `step_execution` trạng thái `running` trước. Thêm UNIQUE `(transaction_id, posting_kind)` trên ledger.

**BE-C2. Engine confirm/verify không xác thực và có IDOR** (`internal/transfer/service.go:515-540`, `internal/api/transaction_engine_handler.go:66-127`)
- `ExecuteEngineConfirm` và `ExecuteEngineVerify` nhận PIN, OTP, `SenderUserID` nhưng không dùng. Chúng chỉ tra `GetTransactionByWorkflow` rồi trả `ref_code`, `amount`, `fee`, `status` của bất kỳ giao dịch nào nếu biết UUID.
- `ExecuteEngineRequest` gọi `engine.Start` ngay (dòng ~390), nên tiền di chuyển mà không cần PIN/OTP. `AuthRequired` chỉ là nhãn trả về, server không enforce, mâu thuẫn với spec `offer_auth_matrix`. Backend không có route PIN nào.
- Lỗi DB bị nuốt và đổi thành "không tìm thấy giao dịch". Chuỗi lỗi này không nằm trong `errTable`, nên trả 500.
- Fix: workflow dừng ở step AUTH chờ xác thực. Confirm verify PIN/OTP bằng hash, có rate-limit và khóa sau N lần sai. Kiểm tra `initiated_by == user` (404 nếu khác).

### High

**BE-H1. Idempotency-Key toàn cục** (migration 000002:57; `transfer/service.go:141-149,369-386`; `wallet/service.go:121-128`; `moneyrequest/service.go:233-241`)
- User B dùng lại key của A sẽ nhận kết quả giao dịch của A. Cùng user đổi payload vẫn nhận kết quả cũ, không có 409.
- Khi client không gửi key, `ExecuteEngineRequest` tự sinh UUID nên không idempotent. Retry mạng sẽ tạo giao dịch thứ hai.
- Fix: UNIQUE `(initiated_by, idempotency_key)` kèm hash request, trả 409 nếu khác. Bắt buộc key ở mọi endpoint ghi tiền.
- Thêm: `/money-requests`, `/money-requests/split`, `/vaults` không thấy `RequireIdempotencyKey` (`router.go:128-129`). **[chưa xác minh]**

**BE-H2. Offer limit bị TOCTOU** (`internal/offerguard/offerguard.go:92-121`, gọi từ `transfer/service.go:315`)
- SUM volume, so với limit, rồi INSERT ở tx khác. N request song song với key khác nhau đều pass. Throttle `IntervalSeconds` cũng bị lách.
- Mốc ngày/tháng dùng timezone server, không cố định giờ VN.
- Fix: `pg_advisory_xact_lock(hash(user,type))` hoặc khóa hàng counter trong cùng tx với INSERT.

**BE-H3. Constraint chống double-reversal chỉ có trong comment** (`reversal/reversal.go:106,126-172`, `queries/transactions.sql:63-66`)
- Không có `one_reversal_per_txn` hay `UNIQUE(subject_type, subject_id)` trong migration.
- Hiện còn chặn được nhờ `idempotency_key = "rev-<id>"`, nhưng lỗi rơi ra là 23505 thô và approval có thể kẹt.
- Executor không kiểm tra `orig.Status == completed` và bỏ qua số hàng của `MarkTransactionReversed`.
- Fix: partial UNIQUE `transactions(reversal_of)`, partial UNIQUE trên approval pending, `FOR UPDATE` giao dịch gốc và kiểm tra `n == 1`.

**BE-H4. Reversal/compensate khi receiver đã tiêu hết tiền** (`reversal.go:153-166`, `handlers.go:113-116`)
- `ErrInsufficientBalance` làm rollback cả finalize, approval kẹt `pending`. Compensate coi đây là lỗi terminal, workflow failed mà tiền vẫn ở receiver.
- Fix: thêm trạng thái `reversal_failed` hoặc `needs_manual` kèm audit. Cân nhắc tài khoản nợ (receivable) hoặc hold.

**BE-H5. Money request bị đánh dấu `paid` quá sớm** (`moneyrequest/service.go:244-256`)
- `paid` được set trong tx tạo transaction, trước khi MOVE_FUNDS chạy bất đồng bộ. Workflow fail thì request vẫn `paid`.
- Fix: giữ `pending` đến khi transaction `completed`, hoặc thêm trạng thái `paying` và quay lại `pending` khi fail.

**BE-H6. Access token không thu hồi, không kiểm tra trạng thái; admin không có RBAC** (`api/middleware.go:16-34`, `auth/jwt.go`, `api/admin_handler.go:25-42`, `auth/admin.go`)
- User bị khóa vẫn dùng access token tới 15 phút và vẫn refresh được. Logout chỉ thu hồi refresh family.
- `RequireAdmin` cho mọi admin mọi quyền, kể cả `UpdateAISetting`, tạo AI key, reversal. Admin không có refresh. Chung `JWT_SECRET` với user, chỉ tách bằng `aud`.
- Fix: thêm jti hoặc token version và check status (cache Redis). RBAC theo role cho từng route. Dùng secret riêng cho admin.

**BE-H7. Cấu hình mặc định yếu** (`platform/config/config.go:64,81,95`, `cmd/server/main.go:117`, `api/router.go:45`, `approval_handler.go:43-45`)
- `APP_ENV` mặc định `dev`. Ở dev, `otp_dev` được trả trong response `/approvals/:id/challenge`, và JWT secret ngắn vẫn được chấp nhận.
- `MOCK_BANK_SECRET` mặc định `dev-mock-bank-secret`.
- Danh sách env hợp lệ (comment) là `dev|ci|demo`, nhưng `main.go` so sánh `!= "production"`. Hậu quả: mọi env hiện dùng gateway Mock, không có đường bật gateway thật có chủ đích.
- `godotenv.Load()` chạy ở mọi env.
- `/internal/mock-bank/callback` mount ở mọi env, không rate-limit, không IP allowlist. Ai biết secret mặc định có thể nạp tiền giả.
- Fix: `APP_ENV` bắt buộc và whitelist. Cấm secret mặc định ngoài dev. Chỉ mount mock-bank khi không phải production. Chỉ load `.env` khi dev.

**BE-H8. Brute-force và rate limit yếu** (`api/middleware.go:39-54`, `router.go:99-102`, `auth/service.go:73-92`)
- Rate limit theo IP + route, 5 lần/phút. Không khóa theo tài khoản, không đếm lần sai.
- Không thấy `SetTrustedProxies`, nên có thể giả mạo X-Forwarded-For. Rate limit fail-open khi Redis lỗi.
- `/auth/logout`, mock-bank, `/v1/files`, `/v1/admin/*` không có rate limit.
- Login với phone không tồn tại trả sớm (không băm) nên timing lộ user enumeration, cả admin login. Register lộ `ErrPhoneTaken`.

**BE-H9. KYC upload và ảnh CCCD** (`kyc/service.go:41-53`, `kyc_handler.go:58`, `storage/cloudinary.go`, `storage/storage.go:119-141`, `api/admin_handler.go:141-154`)
- Chỉ kiểm tra magic bytes 2-4 byte đầu, không decode ảnh. Key luôn là `id_card.jpg` dù là PNG. Không `MaxBytesReader`, chỉ có `LimitReader`.
- Cloudinary upload mặc định `type=upload` là công khai theo URL. Ai biết URL tải được ảnh CCCD mà không cần chữ ký HMAC của hệ thống.
- Signed URL ký bằng chính `JWT_SECRET`, không gắn với admin nào, không audit khi cấp link. `ServeFile` thiếu `X-Content-Type-Options: nosniff`.
- File mồ côi không được dọn (requestID lưu trữ khác ID dòng DB).
- Fix: upload `authenticated/private`, dùng khóa ký riêng, thêm nosniff và audit, `image.DecodeConfig` kèm giới hạn pixel, `MaxBytesReader`.

### Medium

- **Self-approval không chặn tường minh** (`approval/service.go:221-251`). `eligibility()` chỉ kiểm tra role checker và level. Chỉ phụ thuộc UNIQUE `(org_id, user_id)` (cần xác minh migration 000005). Nên thêm `checkerID != req.CreatedBy`.
- **Không kiểm tra lại số dư/hạn mức khi MOVE_FUNDS** sau khi duyệt. `orgTxnLimit` chỉ kiểm tra lúc tạo. **[chưa xác minh]**
- **`human_approval` chỉ gắn cho ví org.** Service user-wallet có step này sẽ lỗi (`approval/service.go:94-99`).
- **AdminDecide** (`approval/admin.go`): không OTP, không rate limit, không RBAC, không ghi IP. KYC review chỉ cần 1 admin, lý do reject là optional.
- **AI key lưu plaintext** trong `ai_keys.api_key`. Không audit khi tạo/đổi key hoặc setting.
  - `admin_ai.go:127` so sánh chuỗi `"no rows in result set"` thay vì `errors.Is(pgx.ErrNoRows)`.
  - `UpdateAIKey` thiếu field `is_active` sẽ ngầm vô hiệu hóa key.
  - `advisor.New("")` trả mock im lặng.
  - Model `claude-sonnet-5` bị hardcode, bỏ qua `model_name` trong DB.
  - Ảnh CCCD gửi sang LLM bên thứ ba cần cân nhắc pháp lý.
- **Ledger `Post`** không kiểm tra `status` và `currency` ví (`ledger.go:83-99`). Không chống overflow int64 (`ledger.go:39-59`, `fee.go:26`). `ExecuteEngineRequest` không áp `wallet.TxnLimit`.
- **Fee**:
  - Làm tròn xuống, chưa chốt trong spec.
  - `fee_policies` thiếu CHECK và UNIQUE `(txn_type, kyc_status, version)`.
  - Không chặn `fee > amount`.
  - `Calc` nuốt giá trị âm thành 0.
- **Outbox** (`outbox/relay.go:63-83`): at-least-once, không có dead-letter hay attempts. Event độc chặn đầu hàng đợi. Index sắp theo `available_at` nhưng query `ORDER BY id`. `PumpLocal` mark published trước khi xử lý (chỉ dev/test, nên chặn ở production).
- **`TransferToUser` (legacy) bỏ qua offerguard** (`transfer/service.go:88-160`). Nếu endpoint còn mở thì lách được limit.
- **Workflow**:
  - Cần xác nhận trạng thái `compensation_failed` và alert (`engine.go:301-310`).
  - `dispatchCompensate` dùng Attempt cố định 101. **[chưa xác minh]**
  - `consumer.go:77` dùng `time.Sleep(1s)` thay vì `select ctx.Done()`.
  - Lỗi mạng khi tải ảnh KYC bị coi là terminal, không retry.
- **Fraud** (`fraud/handler.go`): `HourOfDay` dùng giờ server. Goroutine advisory ghi đè cột `reasons`, có thể mất dữ liệu.
- **HTTP server và header**:
  - Thiếu Read/Write/IdleTimeout (`main.go:77-81`), CORS, HSTS, nosniff, body limit toàn cục.
  - `healthz` trả `err.Error()` của Postgres/Redis cho người lạ.
- **Refresh token**: không kiểm tra `users.status`. `X-Device-Id` do client tự gửi và không ràng buộc. Không giới hạn số family/thiết bị. Audit bị nuốt bằng `_ =`.
- **OTP**: hash gắn `userID:purpose` thay vì challenge id. Không có giới hạn tổng theo user. `Decide` đốt OTP trước khi check eligibility.
- **User lookup** cho phép liệt kê số điện thoại (20/phút/user, đăng ký miễn phí).

### Low

- **Seed không có guard môi trường.** `.env` đang trỏ DB Neon thật. Seed tạo mật khẩu cố định `Demo@123` cho user demo và `E2ETest@123` cho 2 admin e2e, nên chạy nhầm sẽ tạo admin với mật khẩu đã biết (`cmd/seed/main.go`).
- `docker-compose.yml`: Redis không mật khẩu, mở cổng ra host, thiếu `JWT_SECRET` cho profile app.
- `.gitignore` không có `data/` (ảnh KYC local sau này có thể bị commit).
- `NewRefCode` 48 bit, bỏ qua lỗi `rand.Read`, không retry khi trùng.
- `vault.go:74-76` cắt label theo byte, hỏng UTF-8 tiếng Việt. Không validate `target < 0`.
- `qr` receiver_token dùng lại được trong 10 phút. Chưa xác nhận `hmac.Equal`.
- `recon.go`: `buildReport` và LLM chạy trước khi nhận `ErrAlreadyRan`. `day` chưa chuẩn hóa timezone.
- Mật khẩu: không có yêu cầu độ phức tạp. Login không cap độ dài (nguy cơ DoS băm).
- Lỗi bị nuốt rải rác: `uuid.Parse` bỏ lỗi trong handler, `json.Unmarshal` bỏ lỗi (`approval/service.go:126`).
- `otp_challenges` không có job dọn dẹp.
- Có hàm `WalletHandler.Topup` nhưng không thấy route mount (hàm mồ côi hoặc route bị bỏ). **[chưa xác minh]**

### Migration / DB – constraint thiếu (gom vào migration 000014)

| Vấn đề | Vị trí |
|---|---|
| UNIQUE `(initiated_by, idempotency_key)` thay cho UNIQUE toàn cục | 000002:57 |
| `ledger_entries`: thiếu UNIQUE chống post lặp và CHECK `wallet_id OR account_id NOT NULL` | 000002:67-80 |
| Cân bằng debit = credit chỉ kiểm ở Go. Cần constraint trigger DEFERRABLE | 000002 |
| `transactions`: thiếu CHECK `sender <> receiver` | 000002:51-64 |
| `transactions.reversal_of`: thiếu partial UNIQUE | 000002:60 |
| `approval_requests`: thiếu UNIQUE theo subject (pending) | 000005:65-85 |
| `fee_policies`: thiếu CHECK và UNIQUE | 000004:47-57 |
| `wallets.currency` không CHECK, `ledger_entries` không lưu currency | 000002 |
| Thiếu index `(initiated_by, type, created_at)` cho offerguard/fraud | `offer_guard.sql:22-35` |
| `outbox`: thiếu `attempts/last_error`, index không khớp `ORDER BY id` | 000003:69-78 |

Điểm đã xác nhận là đúng:
- Lock ví theo thứ tự id.
- CHECK `balance >= 0` cho ví không phải system.
- Trigger append-only (000007).
- Guard chuyển trạng thái.
- `GetTopupByRefCodeForUpdate`.
- Không dùng float cho tiền.
- Race approve/cancel/double-approve xử lý tốt (`FOR UPDATE` + UNIQUE action).
- Refresh rotation có phát hiện tái sử dụng.

---

## 2. Lỗ hổng – Admin (`hpay-admin`) và Mobile (`hpay-mobile`)

### Critical / High

1. **PIN `'123456'` hard-code mọi giao dịch** (CRITICAL):
   - `app/(main)/transfer/confirm.tsx:53`.
   - `src/queries/useTransactionQueries.ts:155,166,179`.
   - `src/queries/useVaultQueries.ts:81`.
   - App tự gửi PIN cố định. Kết hợp BE-C2, xác thực giao dịch hiện vô hiệu ở cả hai đầu.
2. **Khóa app so PIN cứng ở client** (HIGH): `SecurityWrapper.tsx:111-112` (`pinInput === '123456'`, comment "Mock"). Không có lockout. Sinh trắc chỉ `setLocked(false)`.
3. **Double-submit transfer** (HIGH):
   - Guard `transfer.isPending` dùng mutation `useTransfer`, trong khi submit thực dùng `engine.requestTransaction/confirmTransaction`, nên `isPending` luôn false.
   - Key `useMemo` bị tạo lại khi remount. Bước confirm không có idempotency key.
   - Fix: khóa nút bằng `isRequesting || isConfirming` kèm cờ `useRef`. Lưu key theo transaction id.
4. **Admin không chặn route theo role** (`router.tsx:36-56`, `RequireAdmin.tsx`):
   - Chỉ kiểm tra có token. Viewer vào được `/orgs`, `/ai-settings`, `/services`, `/offers`, `/recon`, `/audit`.
   - `can()` chỉ dùng ở `KYCDetailPage` và `MyTasksPage`. Các quyền `recon.run`, `orgs.edit`, `ai.manage`, `fraud.resolve`, `admins.manage` khai báo nhưng không dùng. Kết hợp BE-H6.
5. **Mock data che lỗi thật** (HIGH):
   - `useReconQueries.ts:107-112,140-143` (fallback `?? 2173`, `duration_ms || 4200`).
   - `useAIQueries.ts:20-79` (chuỗi giả giống API key).
   - `useOrgsQueries.ts`, `useAuditQueries.ts:54`.
   - `DashboardPage.tsx:7-17` (KPI hoàn toàn mock).
   - `catch {}` nuốt cả 401/500 và trả số giả. Operator tưởng đang xem số thật. Chỉ bật mock khi `import.meta.env.DEV`.
6. **Admin JWT** trong `sessionStorage`, không refresh (MEDIUM). Cân nhắc cookie httpOnly và CSP.

### Medium

- **`Number()/parseInt()` trên số tiền** trái quy ước M06 (`money.ts` dùng BigInt): `confirm.tsx:45`, `vaults/[id].tsx:60`, `vaults/index.tsx:43`, `org/admin.tsx:145`, `org/disburse.tsx:43`.
- **Admin không dùng Idempotency-Key** cho thao tác ghi (duyệt workflow, resolve fraud, chạy recon).
- **Mobile refresh xóa phiên khi lỗi tạm thời** (`refresh.ts:31-40`): mọi `!res.ok` kể cả 5xx/429 đều xóa refresh token. Chỉ nên xóa với 401/403.
- `OfferDetailPage.tsx:181-214`: `Number(e.target.value)` không validate âm/NaN. `OrgsPage.tsx:146` chia `/1000000` theo kiểu float.
- `kyc-draft.ts` lưu draft trong SecureStore (giới hạn dung lượng, cần xem có lưu ảnh không). **[chưa xác minh]**

### Low / hạ tầng

- `hpay-mobile/app.config.ts:10`: env `dev` trỏ server thật `https://hpay.demaciahieu.store`. Staging/demo là placeholder `HPay.example.com`. Thiếu `ios.bundleIdentifier` và `android.package`.
- `hpay-admin/e2e/.auth/sessions.json` chứa JWT thật của tài khoản e2e (đã `.gitignore`), nên xóa sau khi chạy. Mật khẩu e2e hard-code trong `e2e/fixtures/auth.ts`.
- File thừa cần dọn: `hpay-backend/hpay-server-local.exe`, `integration_test.exe`, `hpay-backend/tmp/go-build*`, `hpay-docs/push-to-github.ps1`.
- `ServicePipelineModal.tsx:66`: URL mẫu `https://api.bankpartner.vn/...`. Kiểm tra có đang là default gửi lên backend không.
- Admin `api/client.ts`: non-JSON (HTML 502) vẫn trả `res.text()` ép kiểu `T`.
- `pages/Placeholder.tsx` nghi là dead code.
- **Mojibake (UTF-8 bị đọc sai)** thấy ở nhiều file qua PowerShell: `router.tsx`, `LoginPage.tsx`, `transfer/service.go:426-540`, README. **[chưa xác minh]** Có thể chỉ là lỗi hiển thị terminal. Cần mở bằng editor UTF-8. Nếu file thật sự sai thì thông báo lỗi tiếng Việt hiển thị chữ hỏng.

Điểm tốt: access token mobile chỉ trong bộ nhớ, refresh trong SecureStore, refresh single-flight có test, `money.ts` dùng BigInt, mobile có Idempotency-Key cho transfer/request/split/topup/vault/disburse.

---

## 3. Thông tin của dự án khác / dấu vết nguồn tham khảo

| Vị trí | Nội dung | Đề xuất |
|---|---|---|
| `hpay-docs/README.md:16,28`; `architecture/06-service-engine-api-consolidation.html:27` | "TMDS/MoMo", "chuẩn TMDS Enterprise Platform", "TMDS Standard Platform". Đang dùng như tên chuẩn của chính HPay | Đổi thành "HPay Service Engine". Phần tham khảo gom vào mục "Nguồn tham khảo" |
| `hpay-docs/index.html:287` | Footer: "DEH Corporate Authorization Matrix, TMDS Wallet Flow Overview" | Tách khỏi đặc tả HPay |
| `architecture/00-backend-design-v2-base.html:27-30,57-62` | "học từ engine DEH"; `engineVerifyTransaction`, `VERIFY_TRANSACTION` là tên của hệ thống khác | Đổi tên hoặc ghi rõ là tham khảo |
| `architecture/04-feature-extensions.html:2,27,90,95` | "TMDS Wallet… production tại Thái Lan", PromptPay, Node-RED, DipChip, Sails.js/MongoDB/AngularJS | Chuyển vào phụ lục so sánh |
| `planning/master-plan.html:52,103,122`; `master-plan.txt:18,62,76` | "source tmds-wallet-backend (đã đọc code thật)", "claim-lease học TMDS" | Bỏ tên repo nội bộ nếu không phải của bạn |
| `hpay-backend/internal/approval/service.go:150` | Comment "DEH's dry-run idea" | Sửa comment |
| `README.md:21`; `04-feature-extensions.html:25` | "ví Mỹ" nhưng tiền tệ là VND | Ghi rõ "phong cách UI kiểu Venmo/Cash App" |
| `guide/functional-overview.en.html:285-286` | Tier 1/2 ghi "**$200**", trùng ngưỡng ở cả hai tier. Bản `.vi` ghi 5 triệu | Sửa thành 5,000,000 VND |
| `hpay-docs/push-to-github.ps1` | Commit message ghi "34 HTML mockups" (README ghi 42). Tự ghi "xóa file này sau khi chạy" | Xóa file |
| `ref-docs/TMDS-Wallet-Flow-Overview.html` | Tài liệu của hệ khác (đúng chỗ là ref-docs) | Đảm bảo không đưa lên docs công khai |

Kết luận: không thấy stack lạ (Laravel/Django/Stripe/MySQL) nào ngoài phần so sánh TMDS. Module path `github.com/hieumai1507/...` khớp chủ repo.

---

## 4. Mâu thuẫn trong docs và giữa docs với code

| # | Mâu thuẫn | Mức |
|---|---|---|
| 1 | README ghi "5 Core Migrations 000001-000005"; `session-prompt.md:109` ghi 11 migrations; `:116` nhắc "000015". Code có 13 migration (`000001`–`000013`), không có `000015` | Cao |
| 2 | Hạn mức KYC mỗi nơi một số: mockup `01-auth-kyc.html:118` ghi 100 triệu/ngày; seed `primary_pocket_max` L1/L2 là 50 triệu / 200 triệu; `offer_services` mặc định 500 triệu/ngày, 2 tỷ/tháng; doc 05 ghi 100M/txn, 500M/ngày, 2 tỷ/tháng | Cao |
| 3 | Doc 00 `txn_type`/`txn_status` thiếu `processing_async` (code đã có). Seed fee dùng `'disbursement'`, doc ghi `org_disbursement` | TB-Cao |
| 4 | `approval_subject` trong doc 04: `org_transaction\|admin_reversal\|account_lock\|policy_change`; code chỉ có `('org_transaction','reversal')`. `approval_status` thiếu `cancelled` (migration 000011) | TB |
| 5 | Doc 00/01/04 và `index.html` nói **asynq** cho cron/recover_stuck, nhưng `go.mod` (phần require trực tiếp đã đọc) không thấy `asynq`, `golang-migrate`, `testcontainers`. **[chưa xác minh]** Cần `Select-String go.mod -Pattern "asynq\|migrate\|testcontainers"` | TB |
| 6 | Edge case: "E01–E18" (`README:13`, `index.html:272`) và "E01–E20" (nhiều nơi). "37 case" trong khi E(20)+M(12)+A(7) = 39 | Thấp-TB |
| 7 | Số mockup: README 42 màn, push script 34. `07-admin-offer-service.html` không có trong README và thiếu zip tương ứng | Thấp |
| 8 | Endpoint: doc 00 ghi `/auth/register`, `/auth/login` không có `/v1`. Quan hệ giữa `verify` và `confirm` không rõ | TB |
| 9 | Storage: `01:128` ghi local/MinIO, `.env.example` có `STORAGE_PROVIDER=cloudinary` | TB |
| 10 | Roles: docs dùng "L1/L2" nhưng `org_role` chỉ có admin/maker/checker. Tier lưu ở hai nơi (`approval_policies.tiers` JSONB và `approval_policy_tiers.tier_level`) dễ lệch | TB |
| 11 | Phase: `session-prompt.md:120` nhắc F11, `master-plan` chỉ có F1–F10 | Thấp |
| 12 | `push-to-github.ps1`: "giống 3 repo kia" nhưng docs là repo thứ 4 | Thấp |

---

## 5. Lỗ hổng logic trong thiết kế (docs chưa định nghĩa)

1. **Idempotency**: phạm vi (user hay toàn cục), cùng key khác payload thì xử lý sao, key sống bao lâu.
2. **request / confirm / verify**: thiếu mô hình trạng thái.
   - `request` hết hạn sau bao lâu.
   - `confirm` gọi 2 lần, `confirm` sau khi hết hạn.
   - Fee khóa ở `request` hay tính lại ở `confirm`.
   - `verify` đứng ở đâu với hạn mức lớn.
3. **Reversal**:
   - Receiver không đủ số dư để hoàn.
   - Có hoàn fee không, hoàn một phần hay toàn phần, hoàn hai lần.
   - Hạn tối đa để hoàn.
   - Docs nói reversal qua maker-checker nhưng F9 chưa làm.
4. **Maker-checker**:
   - Rule Maker ≠ Checker enforce bằng DB hay chỉ ở code. Tier 2 cần hai người khác nhau.
   - Tiền có được hold trong lúc chờ duyệt 24h không.
   - Checker bị xóa hoặc khóa giữa chừng, approval hết hạn đúng lúc đang decide.
   - "5 challenge fail → khóa approve 15 phút" khóa theo user hay theo approval (theo approval thì DoS được checker).
5. **Ledger**:
   - Thứ tự lock cố định để tránh deadlock chéo (E02).
   - Ví `system` được phép âm, doc §1 nói chung "balance ≥ 0".
   - TRUNCATE và quyền DB riêng cho role app (trigger append-only chưa chặn).
   - Chưa có job đối chiếu `wallets.balance = SUM(ledger_entries)` và cảnh báo.
6. **Fee**: làm tròn, ai trả fee, có hoàn fee khi fail/reversal không, nhiều dòng cùng khớp thì ưu tiên nào (`kyc_status` NULL).
7. **KYC**:
   - Khác biệt `auto_approved` vs `verified` và ngưỡng điểm AI.
   - Hạ cấp KYC khi đang có giao dịch vượt hạn mức mới.
   - Cách tính cửa sổ ngày/tháng (múi giờ nào, có tính pending/failed không).
8. **Money request / split**: người nhận bị khóa, hết hạn đúng lúc đang trả (E19), giới hạn tần suất `remind`.
9. **Fraud**: vùng xám 40–70 gọi LLM nhưng chưa ghi hành vi khi LLM lỗi/timeout (fail-open hay fail-closed) và flag có chặn giao dịch hay chỉ đánh dấu.

---

## 6. Gợi ý cải thiện

**Kiểm thử còn thiếu (ưu tiên)**
- Post ledger 2 lần cùng `txnID`.
- Crash-recovery MOVE_FUNDS (lỗi giữa Commit và `recordAndEmit`, rồi replay).
- Race offerguard (N goroutine cùng user).
- Idempotency chéo user và cùng key khác payload.
- Reversal đồng thời, và reversal khi receiver hết tiền.
- IDOR trên confirm/verify, PIN sai.
- Signed URL hết hạn/giả mạo, path traversal `Local.Get`.
- Property-based cho `fee.Calc`.

**Kiến trúc**
- Tách `ExecuteEngineRequest` (~140 dòng). Gom logic trùng idempotency ở transfer/wallet/moneyrequest thành helper.
- Thay `strings.Contains` để nhận lỗi bằng error code hoặc `errors.Is` (`transfer/service.go:197-203`).
- Chuyển `TxnLimit` hardcode (`wallet/service.go:33-42`) vào bảng policy.
- Chuẩn hóa timezone Asia/Ho_Chi_Minh cho mọi phép tính ngày/tháng.
- Gom goroutine fire-and-forget (`fraud/handler.go:97`) vào worker có graceful shutdown.
- Map `pgx.ErrNoRows` thành 404 và bổ sung `errTable`.

**Frontend**
- Hook dùng chung `useSubmitGuard` cho mọi thao tác tiền.
- Truyền bigint thay `Number()`. Thêm lint rule cấm `Number(`/`parseInt(` trên biến amount.
- Dùng `openapi-fetch` với `schema.d.ts` để compile bắt lệch contract.
- Tách mock khỏi bản build production. Thêm CSP cho admin.

**Vận hành**
- Chạy `govulncheck`, `gosec`. Chạy `gitnexus analyze --pdg` rồi `explain` để rà taint.
- `.gitignore` thêm `data/`. Dọn file `.exe`, `tmp/`, `sessions.json`.
- Dự án chưa là git repo (`D:\hpay` không có `.git`), nên `.gitignore` hiện không bảo vệ gì cả. Nên `git init` và kiểm tra trước khi push.

---

## 7. Thứ tự xử lý đề xuất

1. **Hôm nay**: rotate secret (mục 0.1). Xóa `sessions.json`, `.exe`.
2. **Trước khi chạy thật**: BE-C1, BE-C2, BE-H1, BE-H2, BE-H3. Migration 000014 gom các constraint thiếu. Bỏ PIN mock ở mobile.
3. **Tiếp theo**: RBAC admin (backend + route frontend), ràng buộc `APP_ENV` và secret, vá Cloudinary/KYC, self-approval tường minh, BE-H5.
4. **Sau đó**: sửa docs (mục 3, 4), bổ sung mục "bất biến và ngoại lệ" (mục 5), thêm test (mục 6).
5. **Cần kiểm chứng thêm**: các mục **[chưa xác minh]**, đối chiếu `openapi.yaml` với frontend, đọc `engine.go`/`gateway.go` (SSRF qua URL service cấu hình trong DB), kiểm tra encoding file.
