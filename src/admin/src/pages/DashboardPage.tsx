import { Link } from 'react-router-dom';
import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';

import { useFraudFlags, useKYCQueue } from '@/queries';
import { BarChart } from '@/ui';
import { MOCKS_ENABLED } from '@/lib/env';

// There is no analytics API yet, so the KPI cards and the hourly chart have no
// real data source. In a production build they show an em dash. Demo numbers
// are loaded only in a Vite dev server with VITE_USE_MOCKS=true.
const KPI_LABELS = ['New users (today)', 'Transactions (24h)', 'Volume (24h)', 'Failure rate'];
const HOUR_LABELS = Array.from({ length: 24 }, (_, h) => `${h}h`);

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
  const [kpis, setKpis] = useState<Kpi[]>(EMPTY_KPIS);
  const [volume, setVolume] = useState<number[] | null>(null);
  useEffect(() => {
    if (!(import.meta.env.DEV && MOCKS_ENABLED)) return;
    void import('@/mocks/dashboard').then((m) => {
      setKpis(m.MOCK_KPIS);
      setVolume(m.MOCK_VOLUME);
    });
  }, []);
  const mocked = volume !== null;

  const kycQueue = useKYCQueue('manual');
  const fraudFlags = useFraudFlags('open');

  const alerts = [
    {
      to: '/kyc',
      label: 'Pending KYC',
      count: kycQueue.data?.total,
      loading: kycQueue.isPending,
      error: Boolean(kycQueue.error),
      tone: 'text-amber-600',
      bg: 'bg-amber-50/70 border-l-4 border-amber-500',
    },
    {
      to: '/fraud',
      label: 'Open Fraud flags',
      count: fraudFlags.data?.total,
      loading: fraudFlags.isPending,
      error: Boolean(fraudFlags.error),
      tone: 'text-rose-600',
      bg: 'bg-rose-50/70 border-l-4 border-rose-500',
    },
    {
      to: '/workflows',
      label: 'Workflows stuck > 5 min',
      count: undefined,
      loading: false,
      error: false,
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
                {mocked ? '(sample data, no analytics API yet)' : '(no analytics API yet)'}
              </span>
            </div>
          </Card>
        ))}
      </div>

      {/* volume chart */}
      <Card>
        <div className="flex justify-between items-center mb-3">
          <strong className="text-base font-bold text-slate-900">Volume by hour</strong>
          <span className="text-xs text-slate-500">{mocked ? 'sample data' : 'no data yet'}</span>
        </div>
        {volume ? (
          <BarChart values={volume} labels={HOUR_LABELS} />
        ) : (
          <div className="text-center text-slate-400 py-8 text-sm">—</div>
        )}
      </Card>

      {/* attention panels — live counts */}
      <div>
        <strong className="text-base font-bold text-slate-900">Needs attention</strong>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3.5 mt-2.5">
          {alerts.map((a) => (
            <Link key={a.to} to={a.to} className="no-underline text-inherit block">
              <Card className={a.bg}>
                <div className={`text-2.5xl font-extrabold ${a.tone}`}>
                  {a.loading ? '…' : (a.error ? '—' : (a.count ?? '—'))}
                </div>
                <div className="text-sm font-semibold text-slate-800 mt-1">{a.label}</div>
                <div className="text-xs text-slate-500 mt-0.5">
                  {a.error ? 'Could not load this count' : a.count === undefined && !a.loading ? 'API coming in A3' : 'Click to open the list →'}
                </div>
              </Card>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
