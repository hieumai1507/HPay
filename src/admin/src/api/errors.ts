/** Backend error codes → operator-facing Vietnamese messages. */
const ERROR_MESSAGES: Record<string, string> = {
  ERR_VALIDATION: 'Dữ liệu không hợp lệ, kiểm tra lại các trường.',
  ERR_INVALID_CREDENTIALS: 'Email hoặc mật khẩu không đúng.',
  ERR_ACCOUNT_LOCKED: 'Tài khoản đã bị khóa. Liên hệ superadmin.',
  ERR_RATE_LIMITED: 'Thao tác quá nhanh, thử lại sau ít phút.',
  ERR_UNAUTHORIZED: 'Phiên đăng nhập không hợp lệ.',
  ERR_FORBIDDEN: 'Bạn không có quyền thực hiện thao tác này.',
  ERR_NOT_FOUND: 'Không tìm thấy bản ghi.',
  ERR_CONFLICT: 'Bản ghi đã được xử lý ở nơi khác — dữ liệu vừa được cập nhật.',
  ERR_RECON_ALREADY_RAN: 'Ngày này đã chạy đối soát rồi.',
  ERR_RECON_NOT_FOUND: 'Chưa có kết quả đối soát cho ngày này.',
  ERR_KEY_INACTIVE: 'API key này đang bị vô hiệu hóa.',
  ERR_KEY_NOT_FOUND: 'API key không tồn tại.',
  ERR_LINK_EXPIRED: 'Đường dẫn đã hết hạn.',
  ERR_INVALID_SIGNATURE: 'Chữ ký đường dẫn không hợp lệ.',
  ERR_FILE_NOT_FOUND: 'Không tìm thấy tệp.',
};

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    /** New state returned with 409s so callers can update the row in place (A01). */
    readonly payload?: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export function messageFor(code: string, fallback = 'Có lỗi xảy ra, thử lại sau.'): string {
  return ERROR_MESSAGES[code] ?? fallback;
}
