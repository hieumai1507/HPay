import { useState, useMemo } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useReconRun, useRunRecon, type ReconItem } from '@/queries';
import { Badge } from '@/ui';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';
import { ACTION_PERMISSION, NO_PERMISSION_REASON } from '@/lib/access';
import { formatCount } from '@/lib/money';
import { useAdminStore } from '@/stores/admin';

const columnHelper = createColumnHelper<ReconItem>();

export default function ReconPage() {
  const [date, setDate] = useState('2026-07-07');
  const { data, isLoading, error } = useReconRun(date);
  const runMutation = useRunRecon();
  // UX only: the backend does not enforce admin roles yet (audit BE-H6).
  const canRun = useAdminStore((s) => s.can(ACTION_PERMISSION.recon));

  const { pagination, setPagination, sorting, setSorting } = useTableQueryState({ initialPageSize: 10 });

  const runInfo = data?.run ?? null;
  const items = useMemo(() => data?.items || [], [data?.items]);

  const handleRun = async () => {
    if (runMutation.isPending) return;
    try {
      await runMutation.mutateAsync(date);
      alert(`Reconciliation completed for ${date}.`);
    } catch (e: unknown) {
      alert(`Error: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  const handleExport = () => {
    if (runInfo?.reportUrl) {
      window.open(runInfo.reportUrl, '_blank');
    } else {
      alert(`No Reconciliation file for ${date} yet.`);
    }
  };

  const columns = useMemo(
    () => [
      columnHelper.accessor('status', {
        header: 'Result',
        cell: (info) => renderReconStatusBadge(info.getValue()),
      }),
      columnHelper.accessor('internalRef', {
        header: 'Internal Transaction ID',
        cell: (info) => <span className="font-mono text-slate-800">{info.getValue()}</span>,
      }),
      columnHelper.accessor('externalRef', {
        header: 'Bank statement ref',
        cell: (info) => (
          <span className={`font-mono ${info.getValue() ? 'text-slate-900' : 'text-slate-400'}`}>
            {info.getValue() || '—'}
          </span>
        ),
      }),
      columnHelper.accessor('internalAmount', {
        header: 'Wallet (₫)',
        cell: (info) => <span className="font-mono font-medium">{formatCount(info.getValue())}</span>,
      }),
      columnHelper.accessor('externalAmount', {
        header: 'Bank (₫)',
        cell: (info) => (
          <span className={`font-mono font-medium ${info.getValue() !== null ? 'text-slate-900' : 'text-slate-400'}`}>
            {formatCount(info.getValue())}
          </span>
        ),
      }),
      columnHelper.accessor('timestamp', {
        header: 'Time',
        cell: (info) => <span className="font-mono text-slate-600">{info.getValue() ?? '—'}</span>,
      }),
      columnHelper.display({
        id: 'suggestedAction',
        header: 'Resolve',
        cell: (info) => (
          <button
            type="button"
            onClick={() => alert(`View details and resolve entry ${info.row.original.internalRef}`)}
            className="text-brand font-semibold text-xs hover:underline cursor-pointer bg-none border-none p-0"
          >
            {info.row.original.suggestedAction || 'Xem →'}
          </button>
        ),
      }),
    ],
    []
  );

  const table = useReactTable({
    data: items,
    columns,
    state: {
      pagination,
      sorting,
    },
    onPaginationChange: setPagination,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
  });

  // Integer math: basis points = matched * 10000 / total, shown with 2 decimals.
  const matchedPercent =
    runInfo?.totalCount && runInfo.matchedCount !== null
      ? (Math.round((runInfo.matchedCount * 10000) / runInfo.totalCount) / 100).toFixed(2)
      : null;
  const mismatched = runInfo?.mismatchedCount ?? null;

  return (
    <div className="flex flex-col gap-4">
      {/* Topbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-bold text-slate-900">Reconciliation for {date}</h2>
          {runInfo && (
            <span
              className={`text-xs font-bold px-2.5 py-0.5 rounded-full ${
                mismatched !== null && mismatched > 0 ? 'bg-amber-50 text-amber-700 border border-amber-200' : 'bg-emerald-50 text-emerald-700 border border-emerald-200'
              }`}
            >
              {mismatched === null ? '—' : mismatched > 0 ? `${mismatched} mismatched` : 'Matched 100%'}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2.5">
          <input
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="h-9 px-3 rounded-lg border border-slate-200 text-xs text-slate-900 bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
          <button
            type="button"
            onClick={handleRun}
            disabled={runMutation.isPending || !canRun}
            title={canRun ? undefined : NO_PERMISSION_REASON}
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {runMutation.isPending ? 'Processing...' : '⟳ Run Reconciliation'}
          </button>
          <button
            type="button"
            onClick={handleExport}
            className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50"
          >
            ⤓ Export Excel
          </button>
        </div>
      </div>

      {!canRun && (
        <div className="text-xs text-slate-500">{NO_PERMISSION_REASON} (recon.run)</div>
      )}

      {error && (
        <div role="alert" className="bg-rose-50 border-l-4 border-rose-500 text-rose-700 p-3 rounded-r-lg text-sm">
          Could not load reconciliation data: {error instanceof Error ? error.message : String(error)}
        </div>
      )}

      <div className="flex flex-col gap-4">
        {/* KPI Cards */}
        {runInfo && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Total reconciled</span>
              <div className="text-2xl font-bold text-slate-900">{formatCount(runInfo.totalCount)}</div>
              <span className="text-xs text-slate-500">Transactions</span>
            </div>
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Matched</span>
              <div className="text-2xl font-bold text-emerald-600">
                {formatCount(runInfo.matchedCount)}
              </div>
              <span className="text-xs text-emerald-600 font-medium">{matchedPercent === null ? '—' : `${matchedPercent}%`}</span>
            </div>
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Mismatched</span>
              <div className={`text-2xl font-bold ${mismatched !== null && mismatched > 0 ? 'text-rose-600' : 'text-emerald-600'}`}>
                {formatCount(mismatched)}
              </div>
              <span className="text-xs text-rose-600 font-medium">needs action</span>
            </div>
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Run time</span>
              <div className="text-xl font-bold text-slate-900">
                {runInfo.runAt ? new Date(runInfo.runAt).toLocaleTimeString('en-GB') : '—'}
              </div>
              <span className="text-xs text-slate-500">
                cron daily · {runInfo.durationMs === null ? '—' : `${Math.round(runInfo.durationMs / 100) / 10}s`}
              </span>
            </div>
          </div>
        )}

        {/* AI Summary Card */}
        {runInfo?.aiSummary && (
          <div className="bg-gradient-to-r from-purple-50 to-blue-50 border-l-4 border-purple-600 rounded-xl p-4 border border-slate-200 shadow-xs">
            <div className="text-[11px] font-bold text-purple-600 tracking-wider mb-1.5 uppercase">🤖 AI ROOT-CAUSE SUMMARY</div>
            <div className="text-sm text-slate-700 leading-relaxed">{runInfo.aiSummary}</div>
          </div>
        )}

        {/* Mismatched / Matched Items Table */}
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={items.length}
          emptyMessage="Nothing here yet"
        />
      </div>
    </div>
  );
}

function renderReconStatusBadge(status: ReconItem['status']) {
  switch (status) {
    case 'missing_external':
      return <Badge tone="danger">missing_external</Badge>;
    case 'missing_internal':
      return <Badge tone="warning">missing_internal</Badge>;
    case 'amount_mismatch':
      return <Badge tone="danger">amount_mismatch</Badge>;
    case 'matched':
      return <Badge tone="success">matched</Badge>;
    default:
      return <Badge tone="neutral">{status}</Badge>;
  }
}
