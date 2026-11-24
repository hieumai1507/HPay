/**
 * Features whose backend route does not exist yet (docs/reference/api.md, "Endpoints called
 * by frontends that do not exist"). While a flag is false the screen shows a "not available
 * yet" state and never calls the endpoint, so the app cannot show a fake success.
 * Flip the flag only together with the backend route.
 */
export const FEATURE_AVAILABILITY = {
  /** POST /v1/wallets/topup: handler exists, route not mounted. */
  topup: false,
  /** POST /v1/money-requests/:id/pay: Service.Pay exists, route not mounted. */
  payRequest: false,
} as const;

export const NOT_AVAILABLE_TITLE = 'Feature not available';
export const NOT_AVAILABLE_BODY = 'This feature is still being finished and cannot be used yet. Please check back later.';