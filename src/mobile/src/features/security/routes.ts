/**
 * Routes the money screens send the user to when a payment cannot continue until a security
 * setting exists. Kept in one place so the Set PIN screen (another branch) and the TOTP
 * screens (this branch) can be merged without touching every caller.
 */
export const SECURITY_ROUTES = {
  setPin: '/(main)/security/set-pin',
  totpEnroll: '/(main)/security/totp-enroll',
  totpStatus: '/(main)/security/totp-status',
  transferAmount: '/(main)/transfer/amount',
} as const;