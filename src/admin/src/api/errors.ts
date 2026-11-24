/** Backend error codes → operator-facing English messages. */
const ERROR_MESSAGES: Record<string, string> = {
  ERR_VALIDATION: 'Invalid data. Check the fields and try again.',
  ERR_INVALID_CREDENTIALS: 'Incorrect email or password.',
  ERR_ACCOUNT_LOCKED: 'This account is locked. Contact a superadmin.',
  ERR_RATE_LIMITED: 'Too many attempts. Try again in a few minutes.',
  ERR_UNAUTHORIZED: 'Your session is not valid. Please log in again.',
  ERR_FORBIDDEN: 'You do not have permission to do this.',
  ERR_NOT_FOUND: 'Record not found.',
  ERR_CONFLICT: 'This record was already handled elsewhere. The data has been refreshed.',
  ERR_RECON_ALREADY_RAN: 'Reconciliation has already run for this date.',
  ERR_RECON_NOT_FOUND: 'No Reconciliation result for this date yet.',
  ERR_KEY_INACTIVE: 'This API key is disabled.',
  ERR_KEY_NOT_FOUND: 'API key does not exist.',
  ERR_LINK_EXPIRED: 'This link has expired.',
  ERR_INVALID_SIGNATURE: 'The link signature is not valid.',
  ERR_FILE_NOT_FOUND: 'File not found.',
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

export function messageFor(code: string, fallback = 'Something went wrong. Please try again.'): string {
  return ERROR_MESSAGES[code] ?? fallback;
}
