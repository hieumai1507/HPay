import { QueryClient } from '@tanstack/react-query';

import { isApiError } from '@/src/lib/errors';

/**
 * Queries (GET) may retry on transient failures — safe, no side effects.
 * Mutations (POST/PUT/PATCH/DELETE) never auto-retry: a transfer/topup mutation
 * carries an Idempotency-Key and its own explicit retry-via-poll flow (M02/M03);
 * blindly retrying at the query-client layer would fight that flow instead of
 * helping it. See doc 02 §6.
 */
function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  if (failureCount >= 2) return false;
  if (isApiError(error) && error.status >= 400 && error.status < 500) return false;
  return true;
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: shouldRetryQuery,
      retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 4000),
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: false,
    },
  },
});
