# -*- coding: utf-8 -*-
"""Patch the smoke-tested row in 04-platform-crosscutting.xlsx with real results.
One-off helper used after the AUTO-API smoke test against the live hpay-backend
(see qa/plan/implementation-plan.html section 10, step 4). Not part of the
regeneration flow — generate-test-cases.py remains the source of truth for
fresh rows; this script only edits an already-generated file in place.
"""
import openpyxl
from datetime import date

PATH = r"D:\hpay\hpay-docs\qa\test-cases\04-platform-crosscutting.xlsx"

wb = openpyxl.load_workbook(PATH)
ws = wb["Test Cases"]

target_id = "TC-PLT-006"
row_idx = None
for r in range(2, ws.max_row + 1):
    if ws.cell(row=r, column=1).value == target_id:
        row_idx = r
        break
assert row_idx, f"{target_id} not found"

actual_result = (
    "Server: hpay-backend đang chạy local (role=all, healthz OK, postgres+redis "
    "container healthy). User test mới đăng ký qua POST /v1/auth/register (phone "
    "0999224932). Sub-test A — dùng CHỈ header 'Idempotency-Key' (đúng theo hợp đồng "
    "openapi.yaml #/components/parameters/IdempotencyKey) trên 2 lần gọi POST "
    "/v1/transactions/request {service_code:TOPUP_BANK, amount:100000} với header "
    "giống hệt nhau: nhận về 2 transaction_id KHÁC NHAU (c72a5e02-c1fd-4666-9a92-"
    "68d2eef0bf2c / ref MPEE91A4AC3CD3 vs 7be2d1e5-7081-4dc1-94bf-5d54047cf8c9 / ref "
    "MP9ECB642E2FE2) -> KHONG dedupe theo header. Sub-test B — thêm field JSON "
    "body 'idempotency_key' (field nội bộ, KHÔNG có trong request schema công khai "
    "của openapi.yaml) với cùng giá trị ở 2 lần gọi: nhận về CÙNG transaction_id "
    "(8cf3116f-e72a-4983-b148-18e0324c3c3f) và cùng ref_code (MP6CA671C4BE92) ở cả "
    "2 lần -> dedupe đúng khi dùng body field."
)

notes = (
    "PHÁT HIỆN LỆCH HỢP ĐỒNG API (đáng ghi nhận, không phải lỗi tiền bị mất): "
    "middleware RequireIdempotencyKey (internal/api/middleware.go) chỉ kiểm tra "
    "header 'Idempotency-Key' có mặt và <=64 ký tự rồi set vào gin context "
    "'idempotency_key' — nhưng handler RequestTransaction "
    "(internal/api/transaction_engine_handler.go dòng ~28,49) đọc idempotency key "
    "từ trường JSON body 'idempotency_key' (KHÔNG map với header, và trường này "
    "không xuất hiện trong request schema của /v1/transactions/request trong "
    "api/openapi.yaml). Hệ quả: client tuân thủ đúng 100% openapi.yaml công khai "
    "(chỉ gửi header, đúng field đã document: service_code/amount/receiver_phone/"
    "receiver_id/note/extra_params) vẫn KHÔNG được bảo vệ khỏi double-tap/retry ở "
    "endpoint request — mỗi lần gọi lại sinh transaction mới (idemKey rỗng -> "
    "uuid.NewString() ở internal/transfer/service.go dòng ~344-346). Cơ chế dedupe "
    "UNIQUE constraint (internal/transfer/service.go dòng ~133-147, 357-375) hoạt "
    "động ĐÚNG khi có idempotency_key thật trong body — bản thân logic UNIQUE + "
    "GetTransactionByIdempotencyKey không sai. Đề xuất: (a) cập nhật openapi.yaml "
    "thêm 'idempotency_key' vào request schema công khai, HOẶC (b) sửa handler đọc "
    "giá trị đã set trong context bởi RequireIdempotencyKey (c.Get(\"idempotency_key\")) "
    "thay vì chỉ đọc JSON body — để header tài liệu hoá thực sự có tác dụng như "
    "openapi.yaml mô tả. Không tạo Jira/GitHub issue trong lượt này (ngoài phạm vi "
    "artifact) — ghi nhận tại đây làm bằng chứng smoke test."
)

evidence = (
    "Sub-test A req1={transaction_id:c72a5e02-c1fd-4666-9a92-68d2eef0bf2c, "
    "ref_code:MPEE91A4AC3CD3}; req2={transaction_id:7be2d1e5-7081-4dc1-94bf-"
    "5d54047cf8c9, ref_code:MP9ECB642E2FE2} — 2 response KHÁC NHAU dù cùng header. "
    "Sub-test B req1=req2={\"transaction_id\":\"8cf3116f-e72a-4983-b148-"
    "18e0324c3c3f\",\"ref_code\":\"MP6CA671C4BE92\",\"status\":\"processing\","
    "\"service_code\":\"TOPUP_BANK\",\"amount\":100000,\"fee\":0,\"total\":100000,"
    "\"auth_required\":\"pin\"} — bytewise identical ở cả 2 lần gọi."
)

ws.cell(row=row_idx, column=13, value="Fail")  # Status
ws.cell(row=row_idx, column=14, value=actual_result)  # Actual Result
ws.cell(row=row_idx, column=15, value=evidence)  # Evidence
ws.cell(row=row_idx, column=16, value="Claude (auto)")  # Executed By
ws.cell(row=row_idx, column=17, value=date.today().isoformat())  # Last Run Date
ws.cell(row=row_idx, column=18, value=notes)  # Defect ID / Notes

for c in (13, 14, 15, 16, 17, 18):
    ws.cell(row=row_idx, column=c).alignment = openpyxl.styles.Alignment(wrap_text=True, vertical="top")
ws.row_dimensions[row_idx].height = 220

wb.save(PATH)
print("Updated", target_id, "in", PATH)
