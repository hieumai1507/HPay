import { Link, useRouteError } from 'react-router-dom';

export function ErrorPage({ notFound }: { notFound?: boolean }) {
  const error = useRouteError() as unknown;
  const message = notFound
    ? 'Trang không tồn tại.'
    : error instanceof Error
      ? error.message
      : 'Đã có lỗi không mong muốn.';

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
      <h1 style={{ margin: 0, fontSize: 22, fontWeight: 800 }}>{notFound ? '404' : 'Lỗi'}</h1>
      <p style={{ color: 'var(--n500)', margin: 0 }}>{message}</p>
      <Link to="/dashboard" style={{ marginTop: 12, fontWeight: 600 }}>
        ← Về Dashboard
      </Link>
    </div>
  );
}
