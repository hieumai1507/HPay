/**
 * Error codes defined in the OpenAPI spec.
 * Codes come from src/backend/api/openapi.yaml. When the backend adds a new
 * ERR_* code, add it here in the same PR that consumes it — don't let screens invent copy.
 */

export const DEFAULT_ERROR_MESSAGE = 'Something went wrong. Please try again.';
export const NETWORK_ERROR_MESSAGE = 'No network connection. Please check and try again.';

const ERROR_MESSAGES: Record<string, string> = {
  // auth
  ERR_PHONE_TAKEN: 'This phone number is already registered.',
  ERR_INVALID_CREDENTIALS: 'Incorrect phone number or password.',
  ERR_ACCOUNT_LOCKED: 'Your account is locked. Please contact support.',
  ERR_INVALID_TOKEN: 'Your session is not valid. Please log in again.',
  ERR_TOKEN_EXPIRED: 'Your session has expired. Please log in again.',
  ERR_TOKEN_REUSED: 'Unusual log-in activity detected. Please log in again to protect your account.',
  ERR_RATE_LIMITED: 'You are doing that too quickly. Please try again in a few minutes.',
  // money movement
  ERR_AMOUNT_INVALID: 'Invalid amount.',
  ERR_LIMIT_EXCEEDED: 'The amount exceeds your transaction limit.',
  ERR_IDEMPOTENCY_KEY_REQUIRED: 'Invalid request. Please try again.',
  ERR_INSUFFICIENT_BALANCE: 'Insufficient balance for this transaction.',
  ERR_SELF_TRANSFER: 'You cannot transfer money to yourself.',
  ERR_RECEIVER_NOT_FOUND: 'Recipient not found.',
  ERR_TXN_NOT_FOUND: 'Transaction not found.',
  // timeline failure_code allowlist (backend m3-api-support)
  ERR_FRAUD_BLOCKED: 'This transaction is temporarily held for a safety check. Please contact support.',
  ERR_ACCOUNT_INACTIVE: 'The sender or recipient account is not active.',
  ERR_TRANSFER_FAILED: 'The transaction failed. Any amount already deducted will be refunded.',
  // money requests
  ERR_SELF_REQUEST: 'You cannot create a Money request for yourself.',
  ERR_REQUEST_NOT_FOUND: 'Money request not found.',
  ERR_REQUEST_NOT_PAYABLE: 'This Money request has already been handled.',
  ERR_REQUEST_EXPIRED: 'This Money request has expired.',
  // kyc
  ERR_KYC_ALREADY_PENDING: 'Your KYC application is under review.',
  ERR_KYC_ALREADY_VERIFIED: 'Your account is already KYC verified.',
  ERR_KYC_NOT_FOUND: 'KYC application not found.',
  ERR_ALREADY_REVIEWED: 'This application has already been reviewed.',
  // files
  ERR_INVALID_SIGNATURE: 'This link is not valid.',
  ERR_FILE_NOT_FOUND: 'File not found.',
  ERR_LINK_EXPIRED: 'This link has expired.',
  // org / approvals
  ERR_NOT_ORG_MEMBER: 'You are not a member of this Organization.',
  ERR_FORBIDDEN_ROLE: 'Your role is not allowed to do this.',
  ERR_USER_NOT_FOUND: 'User not found.',
  ERR_MEMBER_EXISTS: 'This user is already a member.',
  ERR_INVALID_POLICY: 'The approval policy is not valid.',
  ERR_ORG_INACTIVE: 'This Organization is suspended.',
  ERR_ORG_NOT_FOUND: 'Organization not found.',
  ERR_NO_POLICY: 'This Organization has not set up an approval policy.',
  ERR_WRONG_LEVEL: 'You are not at the right approval level for this order.',
  ERR_APPROVAL_NOT_FOUND: 'Pending approval not found.',
  ERR_ALREADY_FINALIZED: 'This order has already been completed.',
  ERR_ALREADY_ACTED: 'You have already acted on this order.',
  ERR_APPROVAL_EXPIRED: 'This pending approval has expired.',
  ERR_OTP_COOLDOWN: 'Please wait before requesting another OTP code.',
  ERR_OTP_INVALID: 'Incorrect OTP code.',
  ERR_OTP_EXPIRED: 'This OTP code has expired.',
  ERR_OTP_MAX_ATTEMPTS: 'Too many incorrect OTP attempts.',
  // limits by offer (BUG-0017, BUG-0018)
  ERR_LIMIT_PER_TXN: 'The amount exceeds the limit for a single transaction.',
  ERR_LIMIT_DAILY: 'You have reached your daily transaction limit.',
  ERR_LIMIT_MONTHLY: 'You have reached your monthly transaction limit.',
  ERR_TOO_FREQUENT: 'You are doing that too quickly. Please try again in a few minutes.',
  ERR_AMOUNT_BELOW_MIN: 'The amount is below the minimum for this service.',
  ERR_SERVICE_NOT_IN_OFFER: 'This service is not included in your Plan.',
  ERR_SERVICE_DISABLED: 'This service is turned off in your Plan.',
  ERR_OFFER_INACTIVE: 'Your Plan is currently suspended.',
  // transaction PIN (ADR-0016)
  ERR_PIN_WEAK: 'This PIN is too easy to guess. Use 6 digits with no repeats and no sequences.',
  ERR_PIN_NOT_SET: 'You have not set a PIN yet. Please set a PIN first.',
  ERR_PIN_INCORRECT: 'Incorrect PIN.',
  ERR_PIN_LOCKED: 'Your PIN is locked for 15 minutes after too many incorrect attempts.',
  ERR_PIN_COOLING: 'You recently reset your PIN. For 24 hours, PIN payments are limited to 2,500,000 ₫.',
  ERR_PIN_RESET_RATE_LIMITED: 'You have requested too many PIN reset codes. Please try again in 1 hour.',
  // second factor (ADR-0017)
  ERR_TOTP_NOT_ENROLLED: 'This transaction needs an authenticator app. Turn on two-step verification first.',
  ERR_TOTP_INVALID: 'The authenticator code is incorrect or has already been used.',
  ERR_TOTP_LOCKED: 'Authenticator codes are locked for 15 minutes after too many incorrect attempts.',
  ERR_TOTP_ALREADY_ENABLED: 'An authenticator app is already turned on for this account.',
  // confirming a held transaction (ADR-0014)
  ERR_AUTH_EXPIRED: 'This payment expired. Start again.',
  ERR_AUTH_METHOD_UNAVAILABLE: 'The verification method for this transaction is not available yet.',
  ERR_CREDENTIAL_REQUIRED: 'Enter your verification code to continue.',
  ERR_ALREADY_CONFIRMED: 'This transaction has already been confirmed.',
  ERR_IDEMPOTENCY_KEY_REUSED: 'Invalid request. Please try again.',
  // login lockout (BUG-0016)
  ERR_LOGIN_LOCKED: 'Too many failed log-in attempts. Your account is locked for 15 minutes.',
  // wallets, vaults, QR, requests
  ERR_WALLET_NOT_FOUND: 'Wallet not found.',
  ERR_VAULT_NOT_FOUND: 'Vault not found.',
  ERR_VAULT_LABEL_TAKEN: 'This Vault name is already in use.',
  ERR_VAULT_DIRECTION_INVALID: 'This Vault action is not valid.',
  ERR_QR_INVALID: 'Invalid QR code.',
  ERR_QR_EXPIRED: 'This QR code has expired.',
  ERR_REMIND_TOO_SOON: 'You just sent a reminder. Please try again later.',
  ERR_PAYLOAD_TOO_LARGE: 'The file is too large. Please choose a smaller image.',
  // generic
  ERR_VALIDATION: 'Invalid data.',
  ERR_FORBIDDEN: 'You do not have permission to do this.',
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

/** English message to show the user for any error the API client can throw. */
export function describeError(err: unknown): string {
  if (isApiError(err)) return getErrorMessage(err.code);
  if (isNetworkError(err)) return NETWORK_ERROR_MESSAGE;
  return DEFAULT_ERROR_MESSAGE;
}
