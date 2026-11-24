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

/**
 * The backend serialises Postgres jsonb columns as []byte, which Go's JSON encoder writes as a
 * base64 string ("W10=" is "[]"). Decode that, accept an already-parsed value too, and fall back
 * to the supplied default for null, bad base64 or bad JSON, so one odd row cannot crash a page.
 */
export function decodeJsonColumn<T>(value: unknown, fallback: T): T {
  if (value === null || value === undefined || value === '') return fallback;
  if (typeof value !== 'string') return value as T;
  try {
    const bin = atob(value);
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    return JSON.parse(new TextDecoder().decode(bytes)) as T;
  } catch {
    try {
      return JSON.parse(value) as T; // plain JSON text, in case the API ever stops base64-encoding
    } catch {
      return fallback;
    }
  }
}