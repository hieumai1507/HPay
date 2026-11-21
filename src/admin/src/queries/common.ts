export interface AdminQueryParams {
  page?: number;
  limit?: number;
  name?: string;
  sort?: string;
  order?: 'asc' | 'desc';
  from?: string;
  to?: string;
  status?: string;
  tab?: string;
  [key: string]: unknown;
}

/**
 * Standard shape every paginated admin hook returns.
 *
 * `total` is the SERVER-side row count for the current filter — not the length
 * of `rows` — so tables can derive pageCount for manualPagination. Hooks fall
 * back to `rows.length` when an endpoint does not report a total yet.
 */
export interface Paged<T> {
  rows: T[];
  total: number;
}

export interface PaginatedResponse<T> {
  items?: T[];
  data?: T[];
  total?: number;
  page?: number;
  limit?: number;
  totalPages?: number;
}

/**
 * Builds a URL query string from an AdminQueryParams object, skipping undefined, null, or empty string values.
 */
export function buildQueryString(params?: AdminQueryParams): string {
  if (!params) return '';
  const searchParams = new URLSearchParams();

  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      searchParams.set(key, String(value));
    }
  });

  const str = searchParams.toString();
  return str ? `?${str}` : '';
}
