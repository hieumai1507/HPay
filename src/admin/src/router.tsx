import { lazy, Suspense } from 'react';
import type { ReactNode } from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';

import { LoginPage } from '@/features/auth/LoginPage';
import { RequireAdmin } from '@/features/auth/RequireAdmin';
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
const MyTasksPage = lazy(() => import('@/pages/MyTasksPage'));
const ReconPage = lazy(() => import('@/pages/ReconPage'));
const OrgsPage = lazy(() => import('@/pages/OrgsPage'));
const AISettingsPage = lazy(() => import('@/pages/AISettingsPage'));
const AuditPage = lazy(() => import('@/pages/AuditPage'));

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
      { path: 'fraud', element: page(<FraudPage />) },
      { path: 'workflows', element: page(<WorkflowPage />) },
      { path: 'workflows/:id', element: page(<WorkflowDetailPage />) },
      { path: 'tasks', element: page(<MyTasksPage />) },
      { path: 'recon', element: page(<ReconPage />) },
      { path: 'services', element: page(<ServiceCatalogPage />) },
      { path: 'offers', element: page(<OfferFactoryPage />) },
      { path: 'offers/:id', element: page(<OfferDetailPage />) },
      { path: 'orgs', element: page(<OrgsPage />) },
      { path: 'ai-settings', element: page(<AISettingsPage />) },
      { path: 'audit', element: page(<AuditPage />) },
    ],
  },
  { path: '*', element: <ErrorPage notFound /> },
]);
