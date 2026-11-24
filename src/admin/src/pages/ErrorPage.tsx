import { Link, useRouteError } from 'react-router-dom';

export function ErrorPage({ notFound }: { notFound?: boolean }) {
  const error = useRouteError() as unknown;
  const message = notFound
    ? 'Page not found.'
    : error instanceof Error
      ? error.message
      : 'Something went wrong. Please try again.';

  return (
    <div
      style={{
        height: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 8,
        background: 'var(--main-bg)',
      }}
    >
      <div style={{ fontSize: 40 }}>{notFound ? '🔍' : '⚠️'}</div>
      <h1 style={{ margin: 0, fontSize: 22, fontWeight: 800 }}>{notFound ? '404' : 'Error'}</h1>
      <p style={{ color: 'var(--n500)', margin: 0 }}>{message}</p>
      <Link to="/dashboard" style={{ marginTop: 12, fontWeight: 600 }}>
        ← Back to Dashboard
      </Link>
    </div>
  );
}
