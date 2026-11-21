# Chạy 1 lần trong PowerShell tại D:\hpay\hpay-docs
# Tạo repo rỗng "hpay-docs" trên GitHub trước (giống 3 repo kia), rồi:

git init -b main
git add -A
git commit -m "docs: initial design docs v2.1 (architecture 00-04, ui design spec, 34 HTML mockups)"
git remote add origin https://github.com/hieumai1507/hpay-docs.git   # sửa username nếu khác
git push -u origin main

# Xong thì xóa file này: Remove-Item push-to-github.ps1
