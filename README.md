# HPay

Ví điện tử fintech: backend Go, admin React, mobile React Native (Expo). Một repo duy nhất.

## Cấu trúc

```
hpay/
├── src/                  # TOÀN BỘ CODE
│   ├── backend/          # Go + Gin + PostgreSQL + Redis (module github.com/hieumai1507/hpay-backend)
│   ├── admin/            # React + Vite + TanStack (back-office)
│   └── mobile/           # Expo / React Native (app người dùng)
├── docs/                 # KNOWLEDGE-BASE cho người và agent (xem docs/README.md)
├── .claude/skills/       # Skill dùng chung cho cả repo (nguồn duy nhất)
├── .github/workflows/    # CI: backend-ci.yml, admin-ci.yml
├── CLAUDE.md, AGENTS.md  # Chỉ dẫn cho agent (GitNexus, quy tắc làm việc)
└── .gitignore, .gitattributes
```

## Quy ước

- **Docs trước, code sau.** Thiết kế nằm trong `docs/`; đổi thiết kế thì sửa docs trước.
- **Skill chỉ ở root** (`.claude/skills/`). Không đặt skill trong `src/*`.
- Mỗi app giữ `CLAUDE.md`/`AGENTS.md` riêng cho quy tắc của app đó.
- Không commit `.env`, `data/`, `e2e/.auth/`, file `.exe`. Xem `.gitignore`.

## Chạy nhanh

```bash
# Backend (cần Docker cho Postgres + Redis)
cd src/backend && cp .env.example .env && docker compose up -d && make migrate-up && go run ./cmd/server

# Admin
cd src/admin && npm ci && npm run dev

# Mobile
cd src/mobile && npm ci && npx expo start --web
```

## Tài liệu

Bắt đầu từ [`docs/README.md`](docs/README.md). Kế hoạch sửa lỗi hiện tại: [`docs/planning/PLAN.md`](docs/planning/PLAN.md). Báo cáo audit: [`docs/audit/AUDIT-REPORT.md`](docs/audit/AUDIT-REPORT.md).
