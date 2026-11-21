# HPay Knowledge Base

Nguồn tri thức cho người và agent. **Quy ước: sửa docs trước, code sau.** Code nằm ở `../src/`.

> Trạng thái chuyển đổi: các tài liệu thiết kế cũ còn ở dạng HTML (đã chuyển vị trí, chưa chuyển nội dung sang `.md`). Việc chuyển HTML → MD là Wave 5 trong [`planning/PLAN.md`](planning/PLAN.md). Tài liệu mới viết bằng `.md`.

## Bản đồ thư mục

| Thư mục | Nội dung | Ai dùng |
|---|---|---|
| `architecture/` | Kiến trúc: 00 nền tảng (schema, workflow, maker-checker) · 01 backend · 02 mobile · 03 admin · 04 mở rộng · 05 offer/service factory · 06 service engine · 07 bảng/phân trang | Agent trước khi sửa code |
| `guide/` | Tổng quan chức năng và kỹ thuật (vi/en) | Người mới, onboarding |
| `design/` | UI spec và mockup HTML (tài sản thiết kế, giữ HTML) | Frontend |
| `api/` | Hợp đồng API. Nguồn thật hiện là `src/backend/api/openapi.yaml` | Cả ba app |
| `planning/` | Master plan, session prompt, **PLAN.md** (kế hoạch sửa lỗi) | Điều phối |
| `plans/{backend,admin,mobile}/` | Plan theo tính năng đã làm (implementation-plan, solution-architecture) | Tham khảo lịch sử |
| `specs/` | Spec tính năng mới (`<slug>/spec.md`), sinh từ skill spec-driven | Agent |
| `qa/` | Kế hoạch QA, test case (xlsx), script sinh test case | QA, agent test |
| `audit/` | Báo cáo rà soát (AUDIT-REPORT.md) | Điều phối |
| `handoff/` | Ghi chú giao việc giữa các agent theo wave | Agent |
| `adr/` | Quyết định kiến trúc | Cả nhóm |
| `reference/external/` | Tài liệu của hệ khác (TMDS...). **Chỉ tham khảo, không phải đặc tả HPay.** Bị `.gitignore` | Không dùng làm nguồn đặc tả |

## Thứ tự đọc cho agent mới

1. `../CLAUDE.md` và CLAUDE.md của app đang làm.
2. `architecture/00-*` rồi file kiến trúc của app đó (01/02/03).
3. `planning/master-plan.*` để biết phase và edge case (E/M/A).
4. `audit/AUDIT-REPORT.md` để biết lỗi đã biết trước khi đụng vào module liên quan.

## Quy tắc viết docs mới

- Định dạng `.md`, diagram dùng khối ```` ```mermaid ````.
- Mỗi tính năng: `specs/<slug>/spec.md` rồi `plans/<app>/<slug>/plan.md`.
- Không dùng tên sản phẩm/công ty khác như đặc tả HPay. Tham khảo ngoài đặt ở `reference/`.
