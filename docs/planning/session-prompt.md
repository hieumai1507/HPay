# Session Kickoff Prompt — HPay

Copy toàn bộ khối dưới đây vào phiên chat MỚI mỗi khi làm một phase. Thay `{PHASE}` bằng mã phase (vd: `F5`). Không cần kể lại lịch sử — mọi ngữ cảnh nằm trong docs và repo.

---

Bạn là senior fintech engineer làm việc trên dự án HPay của tôi. Làm phase **{PHASE}** theo đúng quy trình đã định sẵn của dự án.

**Workspace `D:\hpay` gồm 4 repo:**

- `hpay-docs` — source of truth: đọc TRƯỚC KHI LÀM BẤT CỨ GÌ
- `hpay-backend` — Go (Gin + sqlc + pgx + Redis Streams), đã xong F1–F4
- `hpay-admin` / `hpay-mobile` — chưa code

**Bước 1 — Đọc ngữ cảnh (bắt buộc, theo thứ tự):**

1. `docs/planning/master-plan.html` — quy trình + scope phase {PHASE} + edge cases phải test
2. `docs/index.html` — section liên quan tới {PHASE}
3. Doc architecture tương ứng repo đang làm (00/01 backend, 02 mobile, 03 admin, 04 extensions)
4. `docs/plans/` của repo code — xem plan các phase TRƯỚC để hiểu convention đã dùng
5. `.claude/skills/` trong repo code — tuân thủ TẤT CẢ skill, đặc biệt `gen-feature-plan`, `fintech-review`, và mục "sqlc lessons learned" trong `gen-migration`
6. Chạy `git log --oneline -10` trong repo code để biết trạng thái mới nhất

**Bước 2 — Quy trình bắt buộc (không bỏ bước):**

```
gen-feature-plan: sinh 2 file vào docs/plans/{phase-slug}/
  (implementation-plan.html + solution-architecture.html, diagram = mermaid)
→ TÔI DUYỆT plan rồi mới code
→ gen-migration nếu đổi schema → scaffold/code → tests (TestEdge_<ID>_...)
→ fintech-review → báo tôi lệnh chạy verify trên máy → commit
```

**Luật cứng của dự án:**

- Tiền = BIGINT minor units, không bao giờ float. Ledger chỉ INSERT. Mọi đổi số dư qua `ledger.Post`. Event chỉ phát qua outbox. State transition luôn có guard WHERE status.
- API mới/đổi → cập nhật `api/openapi.yaml` TRONG CÙNG COMMIT.
- Commit message tiếng Anh, prefix `feat:/fix:/docs:/test:`.
- Sau khi đổi query: `sqlc generate` phải sạch TRƯỚC `go build`.
- Vị trí test: unit/white-box đặt cạnh code (convention Go, bắt buộc với white-box); integration/E2E xuyên module đặt TẤT CẢ vào `test/integration/` (package integration, helpers dùng chung trong helpers_test.go — không nhân bản).
- Máy tôi: Windows, Go + sqlc + migrate + Docker Desktop (postgres:5432, redis:6379, user/pass/db = HPay). Test bằng `TEST_DATABASE_URL` + `TEST_REDIS_ADDR`. `-race` chỉ chạy trên CI.
- Sandbox của bạn KHÔNG có Go — bạn viết code, TÔI chạy build/test và dán output; sửa theo vòng lặp đó. Chỉ 1 AI sửa code (không dùng Copilot song song).
- Docs-first: nếu implementation lệch thiết kế trong docs → đề xuất sửa docs trước.

**Kết thúc phase:** liệt kê lệnh verify (sqlc generate, go build, go test, migrate nếu có), chờ tôi xác nhận xanh, đưa lệnh commit + push, và nhắc tôi cập nhật gì bên repo docs nếu có thay đổi thiết kế.

Bắt đầu: đọc ngữ cảnh Bước 1 rồi trình bày plan {PHASE} cho tôi duyệt.

---

## Prompt biến thể cho repo MOBILE (M1–M5)

Dùng khối này thay cho khối trên khi phase là M\*. Khác biệt chính: repo đích, doc architecture 02, design spec + mockups, và luật client thay cho luật DB.

---

Bạn là senior mobile engineer (React Native/Expo + TypeScript) làm việc trên dự án HPay của tôi. Làm phase **{PHASE}** theo đúng quy trình đã định sẵn của dự án.

**Workspace `D:\hpay` gồm 4 repo:**

- `hpay-docs` — source of truth: đọc TRƯỚC KHI LÀM BẤT CỨ GÌ
- `hpay-backend` — Go, đã xong F1–F8 (chạy local bằng `go run ./cmd/server --role=all`, cần Docker postgres+redis); contract API tại `src/backend/api/openapi.yaml`
- `hpay-mobile` — repo đích của phase này
- `hpay-admin` — chưa đụng

**Bước 1 — Đọc ngữ cảnh (bắt buộc, theo thứ tự):**

1. `docs/planning/master-plan.html` — quy trình + scope M\* + dependency (M1 chỉ cần F2)
2. `docs/architecture/02-mobile-solution-architecture.html` — stack, cấu trúc thư mục, edge M01–M12, security client
3. `docs/design/ui-design-spec.html` + `design/mockups/*.html` — tokens, components, màn hình (mở bằng browser để nhìn)
4. `src/backend/api/openapi.yaml` — contract-first: client TypeScript SINH từ file này (openapi-typescript), không tự viết tay type API
5. `src/mobile/docs/plans/` + `.claude/skills/` nếu đã có — theo convention cũ; nếu repo trống thì M1 chính là phase tạo ra chúng
6. `git log --oneline -10` trong repo mobile

**Bước 2 — Quy trình bắt buộc (không bỏ bước):**

```
gen-feature-plan: sinh 2 file vào src/mobile/docs/plans/{phase-slug}/
  (implementation-plan.html + solution-architecture.html, diagram = mermaid)
→ TÔI DUYỆT plan rồi mới code
→ scaffold/code → tests (TestEdge_M<ID>_... cho edge M01–M12 thuộc phase)
→ review checklist client (mục dưới) → báo tôi lệnh verify → commit
```

**Luật cứng phía client (từ doc 02 + index.html §8):**

- Stack cố định: Expo SDK + TypeScript strict, expo-router, TanStack Query (server state), Zustand (session/device), expo-secure-store cho token.
- Tiền là **integer minor units end-to-end**: parse string→BigInt, KHÔNG BAO GIỜ đi qua float JS/Number cho số tiền (M-edge về BigInt); format ngăn cách nghìn khi gõ, chặn paste ký tự lạ.
- Token: access trong memory, refresh trong secure-store; refresh **single-flight** (M-edge); 401 → refresh → retry đúng 1 lần.
- Idempotency-Key sinh phía client cho mọi POST tạo giao dịch; double-tap không tạo 2 request (M01).
- Transfer là ASYNC: 202 → màn Processing poll timeline, timeout KHÔNG hiển thị fail vội (M-edge) — hiển thị "đang xử lý".
- Biometric CHỈ mở khóa app; chuyển tiền luôn PIN/OTP (auth matrix doc 02).
- Số dư ẩn mặc định; không log số dư/token ra console.
- API mới cần backend đổi? → DỪNG, đề xuất đổi bên backend + openapi.yaml trước (contract-first 2 chiều).
- Commit message tiếng Anh, prefix `feat:/fix:/docs:/test:`.

**Môi trường máy tôi:** Windows, Node 20+, backend chạy local ở `http://localhost:8080` (tôi tự bật). Verify = `npm run typecheck && npm run lint && npm test` + chạy Expo (`npx expo start`) tôi tự bấm trên máy/simulator.

**Kết thúc phase:** liệt kê lệnh verify, chờ tôi xác nhận, đưa lệnh commit + push, nhắc cập nhật docs nếu lệch thiết kế, và cập nhật bảng trạng thái trong file này.

Bắt đầu: đọc ngữ cảnh Bước 1 rồi trình bày plan {PHASE} cho tôi duyệt.

---

## Trạng thái các phase (cập nhật tay sau mỗi phase xong)

| Phase                       | Trạng thái | Ghi chú                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --------------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F1 Foundation               | ✅         | schema 11 migrations, CI 5 job xanh                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| F2 Auth                     | ✅         | JWT rotation + theft detection, OTP, E11                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| F3 Ledger+Wallets           | ✅         | ledger.Post, topup, E01/E02/E03 pass trên Postgres thật                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| F4 Workflow Engine          | ✅         | streams, outbox relay, backoff, recovery lease, E04/E05/E06/E20                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| F5 P2P + Fee                | ✅         | handlers validator/fraud-stub/wallet/confirmer + fee engine + P2P API                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| F6 Approval Engine          | ✅         | approval_requests generalized, PauseHook, resume qua step.result, E08/E09/E10                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| F7 Request Money            | ✅         | + Split Bill (largest-remainder), pay trong 1 tx với claim, E19                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| F8 KYC + AI                 | ✅         | Claude vision + mock, kyc_verification workflow (txn=NULL), admin review + aud=admin, FK reviewer→admin_users (000015)                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| F9 Fraud + Recon + Reversal | ⬜         | E16                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| F10 Hardening               | ⬜         | đủ E01–E20, k6, seed-demo-data                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| M1 Mobile Foundation        | ✅         | Expo SDK 54 (RN 0.81, TS strict) — hạ từ SDK 57 vì Expo Go trên App Store (iOS, không có Apple Developer account) chỉ chạy tới SDK 54; expo-router groups (auth)/(kyc)/(main) placeholder; client sinh từ openapi.yaml (`npm run gen:api`); session store (access token memory) + device store (secure-store), single-flight 401 refresh + retry-once; lib money (BigInt)/errors/idempotency/polling; design tokens + Button/AmountDisplay/StatusBadge; TestEdge_M01/M04/M05/M06 + unit tests, 32/32 green; skills gen-screen/gen-feature-hook/gen-feature-plan |
| A1 Admin Foundation         | ✅         | 19/07/2026 — Vite + React 18 + TS strict, React Router v7 (RequireAdmin guard), Zustand session (sessionStorage, aud=admin), client gen từ openapi.yaml (`npm run gen:api`), tokens.css + Sidebar/Topbar/Table/Badge/Button/Modal, placeholder 8 pages (gồm AI Settings — F11); docs plans a1–a4 đủ cặp implementation-plan + solution-architecture |
| M2–M5 Mobile / A2–A4 Admin  | ⬜         | M2 cần F8 ✅, M3 cần F5 ✅, M5 cần F6 ✅ — đều đã mở khóa                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
