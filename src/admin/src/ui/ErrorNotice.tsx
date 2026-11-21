import type { ReactNode } from 'react';

/** Visible failure state for a query/mutation. Never render mock data in its place. */
export function ErrorNotice({ error, what }: { error: unknown; what: string }) {
  if (!error) return null;
  const message = error instanceof Error ? error.message : String(error);
  return (
    <div role="alert" className="bg-rose-50 border-l-4 border-rose-500 text-rose-700 p-3 rounded-r-lg text-sm">
      Could not load {what}: {message}
    </div>
  );
}

/** Inline explanation for an action that is disabled because the backend has no endpoint for it. */
export function NotAvailableNote({ children }: { children: ReactNode }) {
  return (
    <div role="note" className="bg-amber-50 border-l-4 border-amber-500 text-amber-800 p-3 rounded-r-lg text-xs">
      Not available yet: {children}
    </div>
  );
}
