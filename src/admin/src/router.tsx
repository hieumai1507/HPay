import { lazy, Suspense } from 'react';
import type { ReactNode } from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';

import { LoginPage } from '@/features/auth/LoginPage';
import { RequireAdmin } from '@/features/auth/RequireAdmin';
import { RequirePermission } from '@/features/auth/RequirePermission';
import { ROUTE_ACCESS } from '@/lib/access';
import type { Permission } from '@/stores/admin';
import { Layout } from '@/ui';

import { ErrorPage } from './pages/ErrorPage';

const DashboardPage = lazy(() => import('@/pages/DashboardPage'));
const KYCPage = lazy(() => import('@/pages/KYCPage'));
const KYCDetailPage = lazy(() => import('@/pages/KYCDetailPage'));
const FraudPage = lazy(() => import('@/pages/FraudPage'));
const WorkflowPage = lazy(() => import('@/pages/WorkflowPage'));
const WorkflowDetailPage = lazy(() => import('@/pages/WorkflowDetailPage'));
const ServiceCatalogPage = lazy(() => import('@/pages/ServiceCatalogPage'));
const OfferFactoryPage = lazy(() => import('@/pages/OfferFactoryPage'));
const OfferDetailPage = lazy(() => import('@/pages/OfferDetailPage'));
const NeedsAttentionPage = lazy(() => import('@/pages/NeedsAttentionPage'));
const MyTasksPage = lazy(() => import('@/pages/MyTasksPage'));
const ReconPage = lazy(() => import('@/pages/ReconPage'));
const OrgsPage = lazy(() => import('@/pages/OrgsPage'));
const AISettingsPage = lazy(() => import('@/pages/AISettingsPage'));
const AuditPage = lazy(() => import('@/pages/AuditPage'));

/** UX-only role guard (the backend does not enforce admin roles yet, audit BE-H6). */
function guarded(anyOf: readonly Permission[], node: ReactNode) {
  return <RequirePermission anyOf={anyOf}>{page(node)}</RequirePermission>;
}

function page(node: ReactNode) {
  return <Suspense fallback={<div style={{ padding: 24, color: 'var(--n500)' }}>Đang tải…</div>}>{node}</Suspense>;
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    path: '/',
    element: (
      <RequireAdmin>
        <Layout />
      </RequireAdmin>
    ),
    errorElement: <ErrorPage />,
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: 'dashboard', element: page(<DashboardPage />) },
      { path: 'kyc', element: page(<KYCPage />) },
      { path: 'kyc/:id', element: page(<KYCDetailPage />) },
      { path: 'fraud', element: guarded(ROUTE_ACCESS['/fraud'], <FraudPage />) },
      { path: 'workflows', element: page(<WorkflowPage />) },
      { path: 'workflows/:id', element: page(<WorkflowDetailPage />) },
      { path: 'attention', element: guarded(ROUTE_ACCESS['/attention'], <NeedsAttentionPage />) },
      { path: 'tasks', element: page(<MyTasksPage />) },
      { path: 'recon', element: guarded(ROUTE_ACCESS['/recon'], <ReconPage />) },
      { path: 'services', element: guarded(ROUTE_ACCESS['/services'], <ServiceCatalogPage />) },
      { path: 'offers', element: guarded(ROUTE_ACCESS['/offers'], <OfferFactoryPage />) },
      { path: 'offers/:id', element: guarded(ROUTE_ACCESS['/offers'], <OfferDetailPage />) },
      { path: 'orgs', element: guarded(ROUTE_ACCESS['/orgs'], <OrgsPage />) },
      { path: 'ai-settings', element: guarded(ROUTE_ACCESS['/ai-settings'], <AISettingsPage />) },
      { path: 'audit', element: guarded(ROUTE_ACCESS['/audit'], <AuditPage />) },
    ],
  },
  { path: '*', element: <ErrorPage notFound /> },
]);
