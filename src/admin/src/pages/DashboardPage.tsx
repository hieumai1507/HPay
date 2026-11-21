import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';

import { useFraudFlags, useKYCQueue } from '@/queries';
import { BarChart } from '@/ui';

const MOCK_KPIS = [
  { label: 'Người dùng mới (hôm nay)', value: '128', delta: '+12%' },
  { label: 'Giao dịch (24h)', value: '2.431', delta: '+5%' },
  { label: 'Volume (24h)', value: '4,82 tỷ ₫', delta: '+9%' },
  { label: 'Tỉ lệ thất bại', value: '0,4%', delta: '-0,1đ' },
];

const MOCK_VOLUME = [
  12, 8, 5, 3, 2, 4, 9, 18, 26, 31, 28, 35, 42, 38, 33, 36, 41, 47, 52, 44, 38, 29, 21, 15,
];
const HOUR_LABELS = MOCK_VOLUME.map((_, h) => `${h}h`);

function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div className={`bg-white border border-slate-200 rounded-xl p-4.5 shadow-xs ${className}`}>
      {children}
    </div>
  );
}

export default function DashboardPage() {
  const kycQueue = useKYCQueue('manual');
  const fraudFlags = useFraudFlags('open');

  const alerts = [
    {
      to: '/kyc',
      label: 'KYC chờ duyệt',
      count: kycQueue.data?.total,
      loading: kycQueue.isPending,
      tone: 'text-amber-600',
      bg: 'bg-amber-50/70 border-l-4 border-amber-500',
    },
    {
      to: '/fraud',
      label: 'Fraud flags đang mở',
      count: fraudFlags.data?.total,
      loading: fraudFlags.isPending,
      tone: 'text-rose-600',
      bg: 'bg-rose-50/70 border-l-4 border-rose-500',
    },
    {
      to: '/workflows',
      label: 'Workflow kẹt > 5 phút',
      count: undefined,
      loading: false,
      tone: 'text-cyan-600',
      bg: 'bg-cyan-50/70 border-l-4 border-cyan-500',
    },
  ];

  return (
    <div className="flex flex-col gap-5">
      {/* KPI row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
        {MOCK_KPIS.map((kpi) => (
          <Card key={kpi.label}>
            <div className="text-xs text-slate-500 font-semibold">{kpi.label}</div>
            <div className="text-2xl font-extrabold text-slate-900 my-1">{kpi.value}</div>
            <div className="text-xs text-emerald-600 font-semibold">
              {kpi.delta}{' '}
              <span className="text-slate-400 font-normal">(minh họa — chưa có analytics API)</span>
            </div>
          </Card>
        ))}
      </div>

      {/* volume chart */}
      <Card>
        <div className="flex justify-between items-center mb-3">
          <strong className="text-base font-bold text-slate-900">Volume theo giờ</strong>
          <span className="text-xs text-slate-500">dữ liệu minh họa</span>
        </div>
        <BarChart values={MOCK_VOLUME} labels={HOUR_LABELS} />
      </Card>

      {/* attention panels — live counts */}
      <div>
        <strong className="text-base font-bold text-slate-900">Cần chú ý</strong>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3.5 mt-2.5">
          {alerts.map((a) => (
            <Link key={a.to} to={a.to} className="no-underline text-inherit block">
              <Card className={a.bg}>
                <div className={`text-2.5xl font-extrabold ${a.tone}`}>
                  {a.loading ? '…' : (a.count ?? '—')}
                </div>
                <div className="text-sm font-semibold text-slate-800 mt-1">{a.label}</div>
                <div className="text-xs text-slate-500 mt-0.5">
                  {a.count === undefined && !a.loading ? 'API sẽ có ở A3' : 'Bấm để mở danh sách →'}
                </div>
              </Card>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
