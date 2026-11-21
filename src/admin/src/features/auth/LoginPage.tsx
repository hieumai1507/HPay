import { useState } from 'react';
import type { FormEvent } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';

import { api } from '@/api/client';
import { ApiError } from '@/api/errors';
import { useAdminStore } from '@/stores/admin';
import type { AdminProfile } from '@/stores/admin';
import { Button, Input, Logo } from '@/ui';

interface LoginResponse {
  admin: AdminProfile;
  access_token: string;
}

export function LoginPage() {
  const token = useAdminStore((s) => s.token);
  const setSession = useAdminStore((s) => s.setSession);
  const navigate = useNavigate();
  const location = useLocation();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const from = (location.state as { from?: string } | null)?.from ?? '/dashboard';

  // Must target `from`, not a hardcoded '/dashboard': onSubmit calls
  // setSession() then navigate(from, {replace:true}) — that setSession()
  // also makes `token` truthy, re-rendering this component with this guard
  // still mounted for an instant. The two navigations race, and a hardcoded
  // '/dashboard' here used to win, silently dropping the operator back to
  // the dashboard instead of the page (and filters) they were on before a
  // mid-session re-login.
  if (token) return <Navigate to={from} replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const res = await api<LoginResponse>('/v1/admin/auth/login', {
        method: 'POST',
        body: { email, password },
      });
      setSession(res.access_token, res.admin);
      navigate(from, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Không kết nối được máy chủ.');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div
      style={{
        height: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'var(--sidebar-bg)',
      }}
    >
      <form
        onSubmit={onSubmit}
        style={{
          background: '#fff',
          borderRadius: 'var(--radius)',
          padding: 32,
          width: 'min(380px, calc(100vw - 48px))',
          display: 'flex',
          flexDirection: 'column',
          gap: 16,
        }}
      >
        <div style={{ textAlign: 'center', marginBottom: 4 }}>
          <Logo size={40} color="var(--brand)" textColor="var(--n900)" showText={false} />
          <h1 style={{ fontSize: 20, fontWeight: 800, margin: '4px 0 0' }}>HPay Admin</h1>
          <p style={{ color: 'var(--n500)', fontSize: 13, margin: '4px 0 0' }}>Đăng nhập nội bộ</p>
        </div>
        <Input
          label="Email"
          type="email"
          autoComplete="username"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
        <Input
          label="Mật khẩu"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {error && <div style={{ color: 'var(--danger)', fontSize: 13.5 }}>{error}</div>}
        <Button type="submit" loading={loading}>
          Đăng nhập
        </Button>
      </form>
    </div>
  );
}
