import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';

import { useFraudFlags, useKYCQueue, useWorkflowInstances } from '@/queries';
import { canAccessRoute } from '@/lib/access';
import { useAdminStore } from '@/stores/admin';

// There is no analytics API yet, so the KPI cards and the hourly chart have no real data source
// and show an em dash. No sample numbers are ever shown.
const KPI_LABELS = ['New users (today)', 'Transactions (24h)', 'Volume (24h)', 'Failure rate'];

interface Kpi {
  label: string;
  value: string;
  delta?: string;
}
const EMPTY_KPIS: Kpi[] = KPI_LABELS.map((label) => ({ label, value: '—' }));

function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div className={`bg-white border border-slate-200 rounded-xl p-4.5 shadow-xs ${className}`}>
      {children}
    </div>
  );
}

export default function DashboardPage() {
  const kpis = EMPTY_KPIS;

  // Only ask for what this role may read: a request the server will refuse just shows an error.
  const can = useAdminStore((s) => s.can);
  const canKyc = canAccessRoute(can, '/kyc');
  const canFraud = canAccessRoute(can, '/fraud');
  const canWorkflow = can('workflow.action'); // GET /v1/admin/workflows needs it (router.go: workflowP)
  const kycQueue = useKYCQueue('manual', canKyc);
  const fraudFlags = useFraudFlags('open', canFraud);
  const stuck = useWorkflowInstances({ stuckOnly: true, limit: 1 }, canWorkflow);

  const alerts = [
    {
      to: '/kyc',
      label: 'Pending KYC',
      allowed: canKyc,
      count: kycQueue.data?.total,
      loading: canKyc && kycQueue.isPending,
      error: Boolean(kycQueue.error),
      tone: 'text-amber-600',
      bg: 'bg-amber-50/70 border-l-4 border-amber-500',
    },
    {
      to: '/fraud',
      label: 'Open Fraud flags',
      allowed: canFraud,
      count: fraudFlags.data?.total,
      loading: canFraud && fraudFlags.isPending,
      error: Boolean(fraudFlags.error),
      tone: 'text-rose-600',
      bg: 'bg-rose-50/70 border-l-4 border-rose-500',
    },
    {
      to: '/workflows',
      label: 'Workflows stuck > 5 min',
      allowed: canWorkflow,
      count: stuck.data?.total,
      loading: canWorkflow && stuck.isPending,
      error: Boolean(stuck.error),
      tone: 'text-cyan-600',
      bg: 'bg-cyan-50/70 border-l-4 border-cyan-500',
    },
  ];
  return (
    <div className="flex flex-col gap-5">
      {/* KPI row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
        {kpis.map((kpi) => (
          <Card key={kpi.label}>
            <div className="text-xs text-slate-500 font-semibold">{kpi.label}</div>
            <div className="text-2xl font-extrabold text-slate-900 my-1">{kpi.value}</div>
            <div className="text-xs text-emerald-600 font-semibold">
              {kpi.delta ?? ''}{' '}
              <span className="text-slate-400 font-normal">
                (no analytics API yet)
              </span>
            </div>
          </Card>
        ))}
      </div>

      {/* volume chart */}
      <Card>
        <div className="flex justify-between items-center mb-3">
          <strong className="text-base font-bold text-slate-900">Volume by hour</strong>
          <span className="text-xs text-slate-500">no data yet</span>
        </div>
        <div className="text-center text-slate-400 py-8 text-sm">—</div>
      </Card>

      {/* attention panels — live counts */}
      <div>
        <strong className="text-base font-bold text-slate-900">Needs attention</strong>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3.5 mt-2.5">
          {alerts.map((a) => {
            const card = (
              <Card className={a.allowed ? a.bg : 'opacity-60'}>
                <div className={`text-2.5xl font-extrabold ${a.tone}`}>
                  {!a.allowed ? '—' : a.loading ? '…' : a.error ? '—' : (a.count ?? '—')}
                </div>
                <div className="text-sm font-semibold text-slate-800 mt-1">{a.label}</div>
                <div className="text-xs text-slate-500 mt-0.5">
                  {!a.allowed ? 'Not available for your role' : a.error ? 'Could not load this count' : 'Click to open the list →'}
                </div>
              </Card>
            );
            // A card the role cannot use is plain text, not a link into a page that would refuse it.
            return a.allowed ? (
              <Link key={a.to} to={a.to} className="no-underline text-inherit block">
                {card}
              </Link>
            ) : (
              <div key={a.to} aria-disabled="true">
                {card}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
