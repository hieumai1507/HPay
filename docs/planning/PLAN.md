# HPay – Kế hoạch sửa lỗi và hợp nhất monorepo

Ngày: 2026-11-21. Căn cứ: `AUDIT-REPORT.md` (gồm phụ lục xác minh).
Trạng thái: **bản nháp chờ bạn duyệt**. Chưa có thay đổi nào trong code.

## 0. Quyết định cần bạn chốt trước khi chạy

Mỗi mục có mặc định tôi đề xuất. Nếu bạn không nói gì, kế hoạch dùng mặc định.

| # | Câu hỏi | Mặc định đề xuất |
|---|---|---|
| D1 | Cấu trúc monorepo | `D:\hpay` là một git repo duy nhất, giữ 3 thư mục `src/backend/`, `src/admin/`, `src/mobile/` và `docs/` (md). Không dùng submodule |
| D2 | Lịch sử git cũ của 3-4 repo con | Không giữ. Khởi tạo repo mới, commit đầu là snapshot đã dọn secret. Nếu cần giữ lịch sử, dùng `git subtree add` (tốn công hơn) |
| D3 | PIN giao dịch | Làm thật: PIN hash bcrypt/argon2 trên server, endpoint đặt/đổi PIN, khóa sau 5 lần sai |
| D4 | Idempotency key | UNIQUE `(initiated_by, idempotency_key)` kèm `request_hash`, trùng key khác payload trả 409 |
| D5 | Docs "tham khảo TMDS/DEH/MoMo" | Gom vào `docs/reference/` và gắn nhãn "tham khảo, không phải đặc tả HPay". Xóa tên repo nội bộ `tmds-wallet-backend` |
| D6 | Mock data ở admin | Chỉ bật khi `import.meta.env.DEV`; production hiện lỗi thật |
| D7 | Hạn mức KYC chuẩn | Bạn phải chọn một bảng số duy nhất (hiện 4 nguồn lệch nhau). Xem mục 5.2 |
| D8 | Docs nói asynq nhưng code dùng goroutine | Mặc định: sửa docs theo code (goroutine + lease). Thêm asynq chỉ khi bạn muốn chạy nhiều replica |
| D9 | `ref-docs/` (tài liệu TMDS, `.docx`) | Giữ ngoài repo (không commit), hoặc để `docs/reference/` nếu bạn có quyền chia sẻ |

---

## 1. Nguyên tắc chạy

1. **Mỗi agent sở hữu một tập file riêng** (bảng ở mục 4). Hai agent không bao giờ sửa cùng file trong cùng wave. Nếu cần sửa file người khác, ghi vào `docs/handoff/<wave>.md` để wave sau xử lý.
2. **Mỗi agent làm trong git worktree riêng**, nhánh `fix/<id>`. Tôi hoặc bạn merge theo thứ tự wave.
3. **Test trước, fix sau** (red → green) cho mọi lỗi tiền và bảo mật. Bằng chứng là output lệnh test, không phải lời khẳng định.
4. **Mỗi PR phải qua cổng**: `go vet`, `golangci-lint`, `go test ./... -race`, `tsc --noEmit`, `eslint`, test frontend, và skill `fintech-review`.
5. **CLAUDE.md của repo yêu cầu GitNexus**: trước khi sửa symbol phải chạy `impact`, trước khi commit phải chạy `detect_changes`. Sau khi hợp nhất monorepo, index cần chạy lại (`node .gitnexus/run.cjs analyze`).
6. **Không agent nào được commit secret.** Quét bằng `gitleaks` trước mỗi merge.

---

## 2. Wave 0 – Nền tảng (làm trước, tuần tự, do bạn hoặc 1 agent)

| ID | Việc | Kiểm tra xong |
|---|---|---|
| W0-1 | **Xoay key** Neon, Cloudinary, JWT. Việc này chỉ bạn làm được (đăng nhập dashboard). Đặt `JWT_SECRET` ngẫu nhiên >= 32 byte | `.env` mới, key cũ bị vô hiệu |
| W0-2 | Viết `.gitignore` ở root: `.env*` (trừ `.env.example`), `data/`, `e2e/.auth/`, `*.exe`, `tmp/`, `node_modules/`, `dist/`, `.expo/`, `playwright-report/`, `test-results/`, `__pycache__/`, `.gotmp/`, `bin/` | `git status` không liệt kê file nhạy cảm |
| W0-3 | Dọn file thừa: `src/backend/*.exe`, `tmp/go-build*`, `scripts/__pycache__`, `e2e/.auth/sessions.json`, `docs/push-to-github.ps1` | Danh sách xóa được bạn duyệt trước |
| W0-4 | `git init`, chạy `gitleaks detect`, commit baseline | gitleaks sạch |
| W0-5 | Cài bộ skill mới (mục 3) vào `.claude/skills/` ở root | `/skills` liệt kê đủ |
| W0-6 | Chạy baseline: `go test ./... -race`, test admin, test mobile, ghi lại số pass/fail **trước khi sửa** vào `docs/handoff/baseline.md` | Có số liệu để so sánh sau |

Bạn xác nhận W0-3 vì đó là thao tác xóa.

---

## 3. Bộ skill mới (đã clone tại `D:\hpay-skills-research`, tất cả MIT)

Pipeline từ requirement mơ hồ đến test-fix loop, ghép từ nhiều nguồn vì không bộ nào đủ trọn chuỗi:

| Bước | Skill | Nguồn | Ghi chú tích hợp |
|---|---|---|---|
| 1. Làm rõ requirement | `interview-me`, `idea-refine` | addyosmani/agent-skills | `interview-me` cần người dùng trả lời trực tiếp, **không chạy trong agent nền** |
| 2. Spec và scope | `spec-driven-development` + mẫu `spec-template.md`, `/speckit.clarify` | agent-skills, spec-kit | Xuất ra **`.md`** trong `docs/specs/<slug>/spec.md` |
| 3. Plan và task | `planning-and-task-breakdown`, `writing-plans`, `/speckit.analyze` | agent-skills, superpowers, spec-kit | Thay `gen-feature-plan` (đang ép xuất HTML) |
| 4. Phản biện thiết kế | `doubt-driven-development` | agent-skills | Chạy với context mới cho thay đổi về tiền/bảo mật |
| 5. Thực thi song song | `subagent-driven-development`, `dispatching-parallel-agents`, `using-git-worktrees` | superpowers | Khớp với cách chia wave ở mục 4 |
| 6. Cài đặt theo TDD | `test-driven-development`, `incremental-implementation` | superpowers, agent-skills | Test `TestEdge_<ID>_*` theo test case |
| 7. Fix theo vòng lặp | `systematic-debugging`, `debugging-and-error-recovery` | superpowers, agent-skills | Quy tắc: không sửa khi chưa có root cause |
| 8. Kiểm chứng | `verification-before-completion`, `/speckit.converge` | superpowers, spec-kit | Không được báo xong khi chưa dán output lệnh test |
| 9. Review và bảo mật | `code-review-and-quality`, `security-and-hardening`, `fintech-review` (giữ, nâng cấp) | agent-skills, của bạn | |
| 10. E2E | `browser-testing-with-devtools`, ý tưởng từ `bmad-qa-generate-e2e-tests` | agent-skills, BMAD | Không cài cả BMAD (cần `uv` và thư mục `_bmad/`, nặng) |

**Skill của bạn giữ nguyên và nâng cấp:**
- `fintech-review`: thêm check: idempotency theo user + request hash, ledger không post trùng, RBAC admin, không dùng `Number()` cho amount, secret mặc định theo env.
- `gen-migration`, `gen-workflow-step`, `scaffold-module`, `gen-api-test`, `seed-demo-data`: giữ, cập nhật đường dẫn và bỏ nhắc "HTML plan".

**Skill cần viết lại:**
- `gen-feature-plan`: xuất `.md`, không còn "BOTH html" bắt buộc, trỏ tới `docs/`.
- Gộp `docs/skills/` và `src/backend/.claude/skills/` thành một nguồn (hash 7/7 khác nhau, chưa so nội dung, cần diff trước).

**Rủi ro tích hợp cần xử lý:**
- Superpowers ép gọi skill trước mọi phản hồi (`using-superpowers`). Chỉ cài các skill cần, bỏ `using-superpowers` hoặc chỉnh để không xung đột với CLAUDE.md.
- Có hai skill TDD (superpowers và agent-skills) và hai skill debug trùng nhau. Chỉ giữ một bản mỗi loại (đề xuất: superpowers cho TDD và debug).
- Mỗi skill của bên thứ ba phải đọc nội dung trước khi bật (không cài mù), và ghi lại commit hash nguồn trong `.claude/skills/SOURCES.md`.

---

## 4. Các wave sửa lỗi

Ký hiệu: **P0** = làm trước khi chạy thật. File thuộc sở hữu agent nào thì chỉ agent đó được sửa trong wave đó.

### Wave 1 – DB và nền tảng tiền (P0, song song 3 agent)

**Agent A1 – Migration 000014 (constraint)**
- Sở hữu: `src/backend/db/migrations/000014_*`, `internal/store/queries/*.sql`, `internal/store/gen/` (sqlc generate).
- Việc:
  1. UNIQUE `(initiated_by, idempotency_key)`, thêm cột `request_hash`, bỏ UNIQUE toàn cục. Cần kiểm tra dữ liệu hiện có trước khi đổi.
  2. `ledger_entries`: UNIQUE chống post lặp theo `(transaction_id, posting_kind, wallet_id/account_id, direction)`, CHECK `wallet_id IS NOT NULL OR account_id IS NOT NULL`. Thêm cột `posting_kind` (forward/compensate).
  3. Constraint trigger DEFERRABLE kiểm tra tổng debit = credit theo `transaction_id`.
  4. Partial UNIQUE `transactions(reversal_of) WHERE reversal_of IS NOT NULL`; partial UNIQUE `approval_requests(subject_type, subject_id) WHERE status='pending'`.
  5. `transactions`: CHECK `sender_wallet_id <> receiver_wallet_id`. `fee_policies`: CHECK (`fixed_fee>=0`, `percent_bp BETWEEN 0 AND 10000`, `min_fee<=max_fee`) và UNIQUE `(txn_type, kyc_status, version)`. `wallets.currency` CHECK.
  6. `wf_status` thêm `compensation_failed`. Index offerguard `(initiated_by, type, created_at)`. `outbox` thêm `attempts`, `last_error`, `dead_at`, index theo `id`.
  7. Chặn TRUNCATE trên `ledger_entries` và `REVOKE` quyền DB thừa cho role app.
- Skill: `gen-migration`. Có file down. Chạy thử up/down/up trên DB sạch.
- Xong khi: migration chạy sạch, `sqlc generate` biên dịch, test hiện có vẫn pass.

**Agent A2 – Cấu hình và bảo mật nền**
- Sở hữu: `internal/platform/config/`, `cmd/server/main.go`, `cmd/seed/main.go`, `internal/api/router.go` (phần mount), `docker-compose.yml`, `.env.example`.
- Việc:
  1. `APP_ENV` bắt buộc, whitelist `dev|ci|demo|production`, thống nhất mọi chỗ so sánh. Bắt buộc `JWT_SECRET >= 32` và `MOCK_BANK_SECRET` riêng khi không phải dev. `godotenv.Load()` chỉ ở dev.
  2. Gateway: chọn HTTP thật hay Mock theo cấu hình rõ ràng, không theo chuỗi `"production"` mơ hồ.
  3. Mock-bank callback chỉ mount khi không phải production. Thêm rate limit và IP allowlist tùy chọn.
  4. Chỉ trả `otp_dev` khi `Env == "dev"`.
  5. `SetTrustedProxies`, HTTP server thêm Read/Write/Idle timeout, `MaxHeaderBytes`, `healthz` không lộ lỗi chi tiết, thêm header bảo mật (nosniff, HSTS) và CORS có allowlist.
  6. Seed: từ chối chạy nếu env không thuộc dev/ci/demo hoặc host DB không phải localhost (cần cờ ép buộc rõ ràng). Mật khẩu seed sinh ngẫu nhiên và in một lần, hoặc lấy từ env.
  7. `docker-compose.yml`: Redis có mật khẩu, bind `127.0.0.1`.
- Xong khi: test cấu hình cho từng env, server từ chối khởi động với cấu hình sai.

**Agent A3 – Hợp nhất repo (monorepo)**
- Sở hữu: root (`.gitignore`, `README.md`, `Makefile` tổng, `.github/`), không đụng code ứng dụng.
- Việc: thực hiện W0-2 đến W0-5; CI duy nhất chạy backend, admin, mobile; `gitleaks` trong CI; cập nhật đường dẫn trong các workflow cũ; chạy lại GitNexus index.
- Xong khi: CI xanh trên nhánh trống.

### Wave 2 – Sửa logic tiền (P0, tuần tự theo phụ thuộc, 2 agent)

Chờ Wave 1 merge xong (A1 phải xong trước B1).

**Agent B1 – Ledger, workflow, idempotency**
- Sở hữu: `internal/ledger/`, `internal/workflow/` (engine, consumer, handlers, recovery), `internal/outbox/`, `internal/transfer/`, `internal/wallet/`, `internal/moneyrequest/`, `internal/api/middleware.go` (phần idempotency).
- Việc theo thứ tự (mỗi việc: viết test fail trước):
  1. **BE-C1**: ghi `step_executions` (claim `running`) và post ledger an toàn khi replay. Dùng `posting_kind` và UNIQUE của A1 để chống post lặp. Test: gọi `ledger.Post` 2 lần cùng `txnID`; mô phỏng crash giữa Commit và `recordAndEmit` rồi replay.
  2. **BE-C2 (phần engine)**: `request` dừng ở bước AUTH, chỉ chạy tiếp sau confirm hợp lệ. Mô hình trạng thái `request → confirm → verify` có hạn dùng, confirm 2 lần an toàn, fee khóa ở `request`.
  3. **BE-H1**: idempotency theo user + `request_hash`, 409 khi khác payload, bắt buộc key trên mọi endpoint ghi tiền (thêm `RequireIdempotencyKey` cho `/money-requests`, `/split`, `/vaults`, round-up). Bỏ tự sinh UUID khi thiếu key.
  4. **BE-H5**: money request giữ `pending`/`paying` đến khi transaction `completed`, quay lại khi fail.
  5. Compensation: dùng `compensation_failed`, gọi `FailTransactionGuarded`, cho recovery sweep quét `compensating`, hỗ trợ retry (bỏ Attempt cố định 101).
  6. Outbox: dead-letter sau N lần, `PumpLocal` chặn ở production, consumer dùng `select ctx.Done()`.
  7. Ledger: kiểm tra `status` và `currency` ví, chống overflow int64, trần amount ở mọi đầu vào.
- Xong khi: toàn bộ test mới pass với `-race`, test hiện có không hỏng, `fintech-review` PASS.

**Agent B2 – Offerguard, fee, reversal, approval**
- Sở hữu: `internal/offerguard/`, `internal/fee/`, `internal/reversal/`, `internal/approval/`, `internal/org/`, `internal/vault/`, `internal/qr/`, `internal/recon/`, `internal/fraud/`.
- Việc:
  1. **BE-H2**: `pg_advisory_xact_lock` trong cùng tx với INSERT; cố định múi giờ Asia/Ho_Chi_Minh.
  2. **BE-H3, H4**: reversal khóa `FOR UPDATE` giao dịch gốc, bắt buộc `status='completed'`, kiểm tra số hàng, trạng thái `reversal_failed`/`needs_manual` khi receiver hết tiền. Cần quyết định nghiệp vụ (D10 bên dưới) trước khi viết.
  3. Self-approval: thêm `checkerID != req.CreatedBy` vào `eligibility()`. Kiểm tra lại số dư và hạn mức tại bước MOVE_FUNDS sau khi duyệt.
  4. Fee: chốt làm tròn, chặn `fee > amount`, không nuốt giá trị âm.
  5. `vault`: cắt label theo rune, validate `target`. `qr`: token dùng một lần hoặc ghi rõ là cho phép dùng lại.
  6. `recon`: kiểm tra `ErrAlreadyRan` trước khi chạy LLM; lease cho cron khi chạy nhiều replica.
  7. `fraud`: giờ theo múi giờ cố định, tránh ghi đè `reasons`; chốt hành vi khi LLM lỗi (fail-open hay fail-closed, D11).
- Xong khi: test race offerguard (N goroutine), reversal đồng thời, reversal khi receiver hết tiền.

> **D10** (cần bạn quyết): reversal khi receiver đã tiêu hết tiền: tạo nợ (receivable), hold số dư, hay chỉ cho phép hoàn khi đủ tiền và báo thủ công? **D11**: fraud/LLM lỗi: chặn giao dịch (fail-closed) hay cho qua và gắn cờ (fail-open)?

### Wave 3 – Xác thực, phân quyền, KYC (P0, song song 2 agent)

**Agent C1 – Auth, PIN, RBAC**
- Sở hữu: `internal/auth/`, `internal/user/`, `internal/api/` (auth_handler, admin_handler, middleware auth, errmap), `internal/gateway/`.
- Việc:
  1. **BE-C2 (phần xác thực)**: PIN thật (D3): bảng/cột hash, endpoint đặt/đổi PIN, verify trong confirm, đếm lần sai và khóa, kiểm tra `initiated_by == user` (404 nếu khác). Bỏ các field PIN/OTP nhận vào mà không dùng. Thêm `ERR_*` vào `errmap.go`.
  2. **BE-H6**: `RequireAuth` kiểm tra `users.status` (cache Redis ngắn hạn), `jti` hoặc token version để thu hồi. Admin có refresh, kiểm tra status mỗi request, RBAC theo `admin_role` cho từng route (đặc biệt `/ai/*`, `/reversals`, `/services`, `/orgs/:id/policies`, KYC review). Secret riêng cho admin.
  3. **BE-H8**: khóa theo tài khoản (phone) và `(phone, IP)` với backoff; chạy bcrypt trên hash giả khi phone không tồn tại (chống timing enumeration); rate limit cho logout, files, admin; cap độ dài mật khẩu; refresh kiểm tra `users.status`; ràng buộc `device_id`.
  4. SSRF: validate URL gateway (allowlist host/scheme `https`, chặn IP private/loopback/link-local/metadata, tắt hoặc giới hạn redirect, cố định method).
  5. Admin AI: mã hóa `api_key` (AES-GCM, khóa từ env), audit mọi thao tác key/setting, dùng `errors.Is(pgx.ErrNoRows)`, sửa `is_active` mặc định, dùng `model_name` từ DB.
  6. Admin decide: audit có IP, KYC review yêu cầu lý do khi reject.
- Xong khi: test IDOR, test brute-force, test RBAC từng route, test SSRF với danh sách IP chặn.

**Agent C2 – KYC và lưu trữ file**
- Sở hữu: `internal/kyc/`, `internal/storage/`, `internal/ai/`.
- Việc: **BE-H9**: `image.DecodeConfig` và giới hạn pixel, `http.MaxBytesReader`, ghi đúng phần mở rộng, Cloudinary upload `authenticated/private` với URL ký, khóa ký riêng (không dùng `JWT_SECRET`), `nosniff` và `Content-Disposition`, audit khi cấp link, dọn file mồ côi, lỗi mạng khi tải ảnh là retryable. Chốt chính sách gửi ảnh CCCD sang LLM bên thứ ba (D12: có đồng ý người dùng, hay chỉ gửi dữ liệu trích xuất).
- Xong khi: test file giả mạo, polyglot, ảnh quá lớn, link hết hạn.

### Wave 4 – Frontend (song song với Wave 3, sau Wave 2 về contract)

**Agent F1 – Contract và OpenAPI**
- Sở hữu: `src/backend/api/openapi.yaml`, `src/admin/src/api/schema.d.ts` và `src/api/client.ts`, `src/mobile/src/api/`.
- Việc: thống nhất endpoint (danh sách lệch ở phụ lục xác minh): quyết định mỗi endpoint là backend thêm hay frontend sửa; cập nhật enum `workflow status`; sinh lại `schema.d.ts`; thêm bước CI kiểm tra openapi khớp route thật; so enum status transaction/money-request/KYC với DB (chưa làm).

**Agent F2 – Mobile**
- Sở hữu: `src/mobile/`.
- Việc: bỏ PIN `'123456'` (màn nhập PIN thật nối với server, theo C1); khóa app dùng xác thực thật, giới hạn số lần thử; `useSubmitGuard` (cờ ref + disable nút) cho mọi thao tác tiền, lưu idempotency key theo transaction id; bỏ `Number()/parseInt()` trên amount (dùng bigint, thêm lint rule); refresh chỉ xóa phiên khi 401/403; `app.config.ts` đổi env `dev` về localhost, thêm `bundleIdentifier` và `package`; kiểm tra KYC draft trong SecureStore.

**Agent F3 – Admin**
- Sở hữu: `src/admin/`.
- Việc: `RequirePermission` cho route nhạy cảm và ẩn nút theo `can()`; tách mock khỏi production (D6) và hiện lỗi thật, thiếu số thì hiện "—"; Idempotency-Key và disable nút khi ghi; bỏ `Number()` cho amount; CSP trong `index.html`; xử lý response không phải JSON; xóa `Placeholder.tsx` nếu xác nhận không dùng; xóa mật khẩu e2e hard-code (đọc từ env).

### Wave 5 – Docs chuyển sang Markdown (song song, độc lập code)

**Agent D1 – Chuyển HTML → MD**
- Sở hữu: `docs/` → `docs/` (root).
- Cấu trúc đề xuất:
  ```
  docs/
    README.md                    (mục lục)
    architecture/00..07-*.md
    guide/{functional,technical}-overview.{vi,en}.md
    planning/{master-plan,session-prompt}.md
    design/ui-design-spec.md
    design/mockups/              (giữ HTML mockup làm tài sản thiết kế; có README)
    qa/{plan,test-cases}/        (xlsx giữ nguyên; script .py giữ)
    specs/                       (spec và plan theo tính năng, từ skill mới)
    handoff/                     (ghi chú giữa các agent)
    reference/                   (TMDS/DEH, gắn nhãn tham khảo)
    adr/
  ```
- Quy ước: diagram dùng ```mermaid trong md; mockup HTML (28KB+ mỗi file) **không** chuyển sang md vì là sản phẩm thiết kế, chỉ có README mô tả. Phải chạy mà không mất nội dung: dùng `pandoc` (hoặc script) rồi **người rà lại bảng, mermaid và liên kết**.
- Xong khi: mọi liên kết nội bộ hoạt động (script kiểm tra), không còn file `.html` trong `docs/` ngoài `mockups/`.

**Agent D2 – Sửa nội dung docs** (chạy sau D1 để tránh sửa hai lần)
- Việc, theo phụ lục và mục 3-5 của `AUDIT-REPORT.md`:
  1. Tách thông tin dự án khác (D5): README, doc 06 (đổi "TMDS Standard Platform" thành "HPay Service Engine"), `index`, doc 00, doc 04, master-plan. Sửa comment "DEH" trong `approval/service.go:150` (đụng code, đưa cho agent sở hữu file).
  2. Số liệu: số migration (13, sau A1 là 14), E01–E20 và "39 case", số mockup, F11 vào master-plan, bản EN "$200" → 5,000,000 VND, "ví Mỹ" → ghi rõ là phong cách UI.
  3. Hạn mức KYC: một bảng duy nhất (D7), các nơi khác tham chiếu.
  4. Enum: `processing_async`, `cancelled`, `reversal`, `compensation_failed`, role L1/L2 map vào `org_role`.
  5. Sửa docs theo code: asynq → goroutine + lease (D8), storage Cloudinary, endpoint thêm `/v1`, quan hệ `verify` và `confirm`.
  6. Viết mục "Bất biến và ngoại lệ": idempotency, reversal, hold số dư khi chờ duyệt, thứ tự lock, ví system được phép âm, job đối chiếu `wallets.balance = SUM(ledger_entries)`, fee (làm tròn, ai trả, hoàn fee), múi giờ cho hạn mức, hành vi LLM lỗi.
- Xong khi: không còn mâu thuẫn trong danh sách mục 4 của báo cáo.

### Wave 6 – Test, kiểm thử theo test case, vòng lặp fix (sau Wave 2-4)

**Agent T1 – Test tích hợp backend**
- Việc: ánh xạ test case (xlsx `qa/test-cases/*`) sang `TestEdge_<ID>_*`. Bổ sung các test còn thiếu: ledger post 2 lần, crash-recovery MOVE_FUNDS, race offerguard, idempotency chéo user và khác payload, reversal đồng thời và receiver hết tiền, confirm với user không sở hữu, signed URL giả mạo, path traversal, property-based `fee.Calc`. Test DB dùng Postgres + Redis thật qua docker compose (hiện repo không dùng testcontainers, không cần thêm).
- Vòng lặp: chạy → thất bại → `systematic-debugging` tìm root cause → sửa → chạy lại, tối đa 3 vòng mỗi lỗi; quá 3 vòng thì dừng và báo cáo người dùng.

**Agent T2 – E2E**
- Việc: Playwright cho admin (đã có `e2e/`), thêm test cho mobile web hoặc Maestro nếu khả thi. Đối chiếu test case xlsx. Đăng nhập e2e dùng tài khoản riêng từ env.

**Agent T3 – Quét tĩnh và hồi quy**
- Việc: `govulncheck`, `gosec`, `gitleaks`, `npm audit`, `gitnexus analyze --pdg` rồi `explain` để rà taint, `detect_changes` so với baseline, báo cáo `docs/handoff/final-verification.md`.

---

## 5. Việc cần bạn quyết hoặc cung cấp

### 5.1 Chỉ bạn làm được
- Xoay key Neon, Cloudinary (W0-1).
- Duyệt danh sách file xóa (W0-3).
- Cung cấp hoặc xác nhận chính sách nghiệp vụ D3, D7, D10, D11, D12.

### 5.2 Hạn mức KYC (D7) – hiện 4 nguồn lệch nhau
| Nguồn | Giá trị |
|---|---|
| Mockup `01-auth-kyc.html:118` | 100 triệu ₫/ngày |
| Seed `OFFER_CUST_L1/L2` (`primary_pocket_max`) | 50 triệu / 200 triệu |
| `offer_services` mặc định | 500 triệu/ngày, 2 tỷ/tháng |
| Doc 05 | 100M/txn, 500M/ngày, 2 tỷ/tháng |
| Bản vi functional overview | 5 triệu (ngưỡng tier) |

Cần bạn chọn bảng chuẩn theo tier KYC (L0/L1/L2): hạn mức mỗi giao dịch, mỗi ngày, mỗi tháng, tối đa số dư.

---

## 6. Phần còn chưa kiểm chứng (ghi rõ để không bị bỏ sót)

Các mục dưới đây agent chưa đọc hoặc chưa chạy. Mỗi mục được giao cho agent nào xử lý.

| Mục | Giao cho |
|---|---|
| So enum status transaction/money-request/KYC với DB, so từng field response | F1 |
| Hai file docs lớn nhất (`00-backend-design-v2-base.html`, `technical-overview.*`) chưa đọc kỹ từng dòng | D2 |
| File xlsx test case, `ref-docs/*.docx` chưa mở | T1, D2 |
| `workflow/engine.go` chỉ mới kiểm vùng compensation; `round_up.go`, `fraud/rules.go`, `fraud/admin.go`, `product` (phần ghi), `moneyrequest/sweeper.go`, `recon/cron.go` chưa đọc | B1, B2 |
| Chữ ký callback bank (`wallet/callback.go`), `hmac.Equal` trong signer QR | B2, C2 |
| `org_handler`: ai được tạo disbursement; role đổi sau khi tạo lệnh | B2 |
| Recon cron khi nhiều replica | B2 |
| `kyc-draft.ts` có lưu ảnh vào SecureStore không | F2 |
| `ServicePipelineModal.tsx:66` URL mẫu có phải default gửi lên backend không | F3 |
| `docs/skills` và `src/backend/.claude/skills`: khác nhau ở nội dung hay chỉ CRLF | W0-5 |
| Mobile `LICENSE`, tên bundle | F2 |
| Chưa chạy bất kỳ test hay ứng dụng nào để xác nhận lỗi bằng thực nghiệm | W0-6, T1 (đây là cách chắc chắn nhất) |

**Đã bác bỏ:** mojibake. Các file là UTF-8 hợp lệ, chỉ là console hiển thị sai, nên không có việc sửa encoding.

---

## 7. Thứ tự và phụ thuộc (tóm tắt)

```
W0 (nền, tuần tự)
 └─ W1: A1 (migration) ─┐ A2 (config) ─┐ A3 (monorepo/CI)   [song song]
        └──────────────┴──────────────┘
 └─ W2: B1 (ledger/workflow/idempotency) → B2 (offerguard/reversal/approval)
 └─ W3: C1 (auth/PIN/RBAC) ‖ C2 (KYC/storage)      (sau W1; C1 trước F2)
 └─ W4: F1 (contract) → F2 mobile ‖ F3 admin       (sau W2/W3 về contract)
 └─ W5: D1 (HTML→MD) → D2 (sửa nội dung)           (độc lập, chạy được ngay sau W0)
 └─ W6: T1 ‖ T2 → T3 (xác nhận cuối, vòng lặp fix)
```

Wave 5 chỉ phụ thuộc W0 nên có thể chạy song song với W1 để tiết kiệm thời gian. Mỗi wave kết thúc bằng một lần merge và chạy lại baseline.

## 8. Rủi ro chính của kế hoạch

- **Migration 000014 đổi UNIQUE của idempotency key trên DB đã có dữ liệu.** Cần kiểm tra DB đang dùng (Neon) có dữ liệu thật không, và có bản sao lưu trước khi áp dụng.
- **Thay đổi luồng `request → confirm`** ảnh hưởng cả mobile lẫn admin; F1 phải chốt contract trước khi F2/F3 làm.
- **Agent nền không hỏi lại được.** Mọi quyết định D* phải chốt trước khi giao việc, nếu không agent sẽ tự đoán.
- **Kết quả audit dựa trên đọc code**, chưa có test chạy thật. Bước W0-6 và T1 là nơi lỗi được chứng minh hoặc bác bỏ.
