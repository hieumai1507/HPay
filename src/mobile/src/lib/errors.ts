/**
 * Error codes defined in the OpenAPI spec.
 * Codes come from src/backend/api/openapi.yaml. When the backend adds a new
 * ERR_* code, add it here in the same PR that consumes it — don't let screens invent copy.
 */

export const DEFAULT_ERROR_MESSAGE = 'Đã có lỗi xảy ra. Vui lòng thử lại.';
export const NETWORK_ERROR_MESSAGE = 'Không có kết nối mạng. Vui lòng kiểm tra lại.';

const ERROR_MESSAGES: Record<string, string> = {
  // auth
  ERR_PHONE_TAKEN: 'Số điện thoại này đã được đăng ký.',
  ERR_INVALID_CREDENTIALS: 'Số điện thoại hoặc mật khẩu không đúng.',
  ERR_ACCOUNT_LOCKED: 'Tài khoản đã bị khóa. Vui lòng liên hệ hỗ trợ.',
  ERR_INVALID_TOKEN: 'Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.',
  ERR_TOKEN_EXPIRED: 'Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.',
  ERR_TOKEN_REUSED: 'Phát hiện đăng nhập bất thường. Vui lòng đăng nhập lại để bảo mật tài khoản.',
  ERR_RATE_LIMITED: 'Bạn thao tác quá nhanh. Vui lòng thử lại sau ít phút.',
  // money movement
  ERR_AMOUNT_INVALID: 'Số tiền không hợp lệ.',
  ERR_LIMIT_EXCEEDED: 'Số tiền vượt hạn mức giao dịch của bạn.',
  ERR_IDEMPOTENCY_KEY_REQUIRED: 'Yêu cầu không hợp lệ, vui lòng thử lại.',
  ERR_INSUFFICIENT_BALANCE: 'Số dư không đủ để thực hiện giao dịch.',
  ERR_SELF_TRANSFER: 'Không thể chuyển tiền cho chính mình.',
  ERR_RECEIVER_NOT_FOUND: 'Không tìm thấy người nhận.',
  ERR_TXN_NOT_FOUND: 'Không tìm thấy giao dịch.',
  // timeline failure_code allowlist (backend m3-api-support)
  ERR_FRAUD_BLOCKED: 'Giao dịch bị tạm chặn để kiểm tra an toàn. Vui lòng liên hệ hỗ trợ.',
  ERR_ACCOUNT_INACTIVE: 'Tài khoản người gửi hoặc người nhận không hoạt động.',
  ERR_TRANSFER_FAILED: 'Giao dịch không thành công. Số tiền (nếu đã trừ) sẽ được hoàn lại.',
  // money requests
  ERR_SELF_REQUEST: 'Không thể tạo yêu cầu thanh toán cho chính mình.',
  ERR_REQUEST_NOT_FOUND: 'Không tìm thấy yêu cầu thanh toán.',
  ERR_REQUEST_NOT_PAYABLE: 'Yêu cầu này đã được xử lý.',
  ERR_REQUEST_EXPIRED: 'Yêu cầu thanh toán đã hết hạn.',
  // kyc
  ERR_KYC_ALREADY_PENDING: 'Hồ sơ KYC đang được xét duyệt.',
  ERR_KYC_ALREADY_VERIFIED: 'Tài khoản đã xác minh KYC.',
  ERR_KYC_NOT_FOUND: 'Không tìm thấy hồ sơ KYC.',
  ERR_ALREADY_REVIEWED: 'Hồ sơ này đã được xét duyệt.',
  // files
  ERR_INVALID_SIGNATURE: 'Liên kết không hợp lệ.',
  ERR_FILE_NOT_FOUND: 'Không tìm thấy tệp.',
  ERR_LINK_EXPIRED: 'Liên kết đã hết hạn.',
  // org / approvals
  ERR_NOT_ORG_MEMBER: 'Bạn không phải thành viên của tổ chức này.',
  ERR_FORBIDDEN_ROLE: 'Vai trò của bạn không được phép thực hiện thao tác này.',
  ERR_USER_NOT_FOUND: 'Không tìm thấy người dùng.',
  ERR_MEMBER_EXISTS: 'Người dùng đã là thành viên.',
  ERR_INVALID_POLICY: 'Chính sách phê duyệt không hợp lệ.',
  ERR_ORG_INACTIVE: 'Tổ chức đang tạm ngưng hoạt động.',
  ERR_ORG_NOT_FOUND: 'Không tìm thấy tổ chức.',
  ERR_NO_POLICY: 'Tổ chức chưa thiết lập chính sách phê duyệt.',
  ERR_WRONG_LEVEL: 'Bạn không ở đúng cấp phê duyệt cho lệnh này.',
  ERR_APPROVAL_NOT_FOUND: 'Không tìm thấy lệnh chờ duyệt.',
  ERR_ALREADY_FINALIZED: 'Lệnh này đã được xử lý xong.',
  ERR_ALREADY_ACTED: 'Bạn đã xử lý lệnh này rồi.',
  ERR_APPROVAL_EXPIRED: 'Lệnh chờ duyệt đã hết hạn.',
  ERR_OTP_COOLDOWN: 'Vui lòng chờ trước khi gửi lại mã OTP.',
  ERR_OTP_INVALID: 'Mã OTP không đúng.',
  ERR_OTP_EXPIRED: 'Mã OTP đã hết hạn.',
  ERR_OTP_MAX_ATTEMPTS: 'Bạn đã nhập sai OTP quá số lần cho phép.',
  // limits by offer (BUG-0017, BUG-0018)
  ERR_LIMIT_PER_TXN: 'Số tiền vượt hạn mức cho một giao dịch.',
  ERR_LIMIT_DAILY: 'Bạn đã vượt hạn mức giao dịch trong ngày.',
  ERR_LIMIT_MONTHLY: 'Bạn đã vượt hạn mức giao dịch trong tháng.',
  ERR_TOO_FREQUENT: 'Bạn thao tác quá nhanh. Vui lòng thử lại sau ít phút.',
  ERR_AMOUNT_BELOW_MIN: 'Số tiền thấp hơn mức tối thiểu của dịch vụ.',
  ERR_SERVICE_NOT_IN_OFFER: 'Dịch vụ này không có trong gói của bạn.',
  ERR_SERVICE_DISABLED: 'Dịch vụ này đang tắt trong gói của bạn.',
  ERR_OFFER_INACTIVE: 'Gói dịch vụ của bạn đang tạm ngưng.',
  // transaction PIN (ADR-0016)
  ERR_PIN_WEAK: 'Mã PIN quá dễ đoán. Dùng 6 số không lặp lại và không liên tiếp.',
  ERR_PIN_NOT_SET: 'Bạn chưa đặt mã PIN. Hãy đặt mã PIN trước.',
  ERR_PIN_INCORRECT: 'Mã PIN không đúng.',
  ERR_PIN_LOCKED: 'Mã PIN bị khóa 15 phút do nhập sai nhiều lần.',
  ERR_PIN_COOLING: 'Vừa đặt lại mã PIN: trong 24 giờ chỉ giao dịch được tối đa 2.500.000 ₫ bằng PIN.',
  ERR_PIN_RESET_RATE_LIMITED: 'Bạn đã yêu cầu mã đặt lại PIN quá nhiều lần. Vui lòng thử lại sau 1 giờ.',
  // second factor (ADR-0017)
  ERR_TOTP_NOT_ENROLLED: 'Giao dịch này cần ứng dụng xác thực. Hãy bật xác thực hai lớp trước.',
  ERR_TOTP_INVALID: 'Mã xác thực không đúng hoặc đã được dùng.',
  ERR_TOTP_LOCKED: 'Mã xác thực bị khóa 15 phút do nhập sai nhiều lần.',
  ERR_TOTP_ALREADY_ENABLED: 'Ứng dụng xác thực đã được bật cho tài khoản này.',
  // confirming a held transaction (ADR-0014)
  ERR_AUTH_EXPIRED: 'Giao dịch đã hết hạn sau 10 phút chưa xác nhận. Vui lòng tạo lại.',
  ERR_AUTH_METHOD_UNAVAILABLE: 'Phương thức xác thực cho giao dịch này chưa khả dụng.',
  ERR_CREDENTIAL_REQUIRED: 'Cần nhập mã xác nhận để tiếp tục.',
  ERR_ALREADY_CONFIRMED: 'Giao dịch này đã được xác nhận trước đó.',
  ERR_IDEMPOTENCY_KEY_REUSED: 'Yêu cầu không hợp lệ, vui lòng thử lại.',
  // login lockout (BUG-0016)
  ERR_LOGIN_LOCKED: 'Đăng nhập sai quá nhiều lần. Tài khoản bị khóa 15 phút.',
  // wallets, vaults, QR, requests
  ERR_WALLET_NOT_FOUND: 'Không tìm thấy ví.',
  ERR_VAULT_NOT_FOUND: 'Không tìm thấy két.',
  ERR_VAULT_LABEL_TAKEN: 'Tên két này đã được dùng.',
  ERR_VAULT_DIRECTION_INVALID: 'Thao tác với két không hợp lệ.',
  ERR_QR_INVALID: 'Mã QR không hợp lệ.',
  ERR_QR_EXPIRED: 'Mã QR đã hết hạn.',
  ERR_REMIND_TOO_SOON: 'Bạn vừa nhắc xong. Vui lòng thử lại sau.',
  ERR_PAYLOAD_TOO_LARGE: 'Tệp quá lớn. Vui lòng chọn ảnh nhỏ hơn.',
  // generic
  ERR_VALIDATION: 'Dữ liệu không hợp lệ.',
  ERR_FORBIDDEN: 'Bạn không có quyền thực hiện thao tác này.',
};

/** Every code that has user-facing copy; the contract test compares it with docs/reference/error-codes.md. */
export const ERROR_CODES_WITH_COPY: readonly string[] = Object.keys(ERROR_MESSAGES);

export function getErrorMessage(code: string | undefined | null): string {
  if (!code) return DEFAULT_ERROR_MESSAGE;
  return ERROR_MESSAGES[code] ?? DEFAULT_ERROR_MESSAGE;
}

/** Thrown by the API client for any non-2xx response with a parsed {code,message} body. */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly requestId?: string;

  constructor(code: string, status: number, requestId?: string, rawMessage?: string) {
    super(rawMessage ?? getErrorMessage(code));
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.requestId = requestId;
  }
}

/** Thrown by the API client when the request never reached the server (offline, DNS, timeout). */
export class NetworkError extends Error {
  constructor() {
    super(NETWORK_ERROR_MESSAGE);
    this.name = 'NetworkError';
  }
}

export function isApiError(err: unknown): err is ApiError {
  return err instanceof ApiError;
}

export function isNetworkError(err: unknown): err is NetworkError {
  return err instanceof NetworkError;
}

/** Vietnamese message to show the user for any error the API client can throw. */
export function describeError(err: unknown): string {
  if (isApiError(err)) return getErrorMessage(err.code);
  if (isNetworkError(err)) return NETWORK_ERROR_MESSAGE;
  return DEFAULT_ERROR_MESSAGE;
}
