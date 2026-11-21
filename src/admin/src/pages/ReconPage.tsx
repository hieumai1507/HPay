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

const columnHelper = createColumnHelper<ReconItem>();

export default function ReconPage() {
  const [date, setDate] = useState('2026-07-07');
  const { data, isLoading } = useReconRun(date);
  const runMutation = useRunRecon();

  const { pagination, setPagination, sorting, setSorting } = useTableQueryState({ initialPageSize: 10 });

  const runInfo = data?.run;
  const items = useMemo(() => data?.items || [], [data?.items]);

  const handleRun = async () => {
    try {
      await runMutation.mutateAsync(date);
      alert(`Đã hoàn tất đối soát cho ngày ${date}!`);
    } catch (e: unknown) {
      alert(`Lỗi: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  const handleExport = () => {
    if (runInfo?.reportUrl) {
      window.open(runInfo.reportUrl, '_blank');
    } else {
      alert(`Đã tải xuống file đối soát đối chiếu ngày ${date}.xlsx (mô phỏng)`);
    }
  };

  const columns = useMemo(
    () => [
      columnHelper.accessor('status', {
        header: 'Kết quả',
        cell: (info) => renderReconStatusBadge(info.getValue()),
      }),
      columnHelper.accessor('internalRef', {
        header: 'Mã GD nội bộ',
        cell: (info) => <span className="font-mono text-slate-800">{info.getValue()}</span>,
      }),
      columnHelper.accessor('externalRef', {
        header: 'Ref sao kê',
        cell: (info) => (
          <span className={`font-mono ${info.getValue() ? 'text-slate-900' : 'text-slate-400'}`}>
            {info.getValue() || '—'}
          </span>
        ),
      }),
      columnHelper.accessor('internalAmount', {
        header: 'Ví (₫)',
        cell: (info) => <span className="font-mono font-medium">{info.getValue().toLocaleString('vi-VN')}</span>,
      }),
      columnHelper.accessor('externalAmount', {
        header: 'Bank (₫)',
        cell: (info) => (
          <span className={`font-mono font-medium ${info.getValue() !== null ? 'text-slate-900' : 'text-slate-400'}`}>
            {info.getValue() !== null ? info.getValue()?.toLocaleString('vi-VN') : '—'}
          </span>
        ),
      }),
      columnHelper.accessor('timestamp', {
        header: 'Thời điểm',
        cell: (info) => <span className="font-mono text-slate-600">{info.getValue()}</span>,
      }),
      columnHelper.display({
        id: 'suggestedAction',
        header: 'Xử lý',
        cell: (info) => (
          <button
            type="button"
            onClick={() => alert(`Xem chi tiết và xử lý bút toán ${info.row.original.internalRef}`)}
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

  const matchedPercent = runInfo?.totalCount
    ? ((runInfo.matchedCount / runInfo.totalCount) * 100).toFixed(2)
    : '100.00';

  return (
    <div className="flex flex-col gap-4">
      {/* Topbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-bold text-slate-900">Đối soát ngày {date}</h2>
          {runInfo && (
            <span
              className={`text-xs font-bold px-2.5 py-0.5 rounded-full ${
                runInfo.mismatchedCount > 0 ? 'bg-amber-50 text-amber-700 border border-amber-200' : 'bg-emerald-50 text-emerald-700 border border-emerald-200'
              }`}
            >
              {runInfo.mismatchedCount > 0 ? `Lệch ${runInfo.mismatchedCount} bút toán` : 'Khớp 100%'}
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
            disabled={runMutation.isPending}
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
          >
            {runMutation.isPending ? 'Đang chạy…' : '⟳ Chạy đối soát'}
          </button>
          <button
            type="button"
            onClick={handleExport}
            className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50"
          >
            ⤓ Xuất Excel
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-4">
        {/* KPI Cards */}
        {runInfo && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Tổng đối chiếu</span>
              <div className="text-2xl font-bold text-slate-900">{runInfo.totalCount.toLocaleString('vi-VN')}</div>
              <span className="text-xs text-slate-500">giao dịch</span>
            </div>
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Khớp</span>
              <div className="text-2xl font-bold text-emerald-600">
                {runInfo.matchedCount.toLocaleString('vi-VN')}
              </div>
              <span className="text-xs text-emerald-600 font-medium">{matchedPercent}%</span>
            </div>
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Lệch</span>
              <div className={`text-2xl font-bold ${runInfo.mismatchedCount > 0 ? 'text-rose-600' : 'text-emerald-600'}`}>
                {runInfo.mismatchedCount}
              </div>
              <span className="text-xs text-rose-600 font-medium">cần xử lý</span>
            </div>
            <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs flex flex-col gap-1">
              <span className="text-xs text-slate-500">Thời gian chạy</span>
              <div className="text-xl font-bold text-slate-900">
                {new Date(runInfo.runAt).toLocaleTimeString('vi-VN')}
              </div>
              <span className="text-xs text-slate-500">cron daily · {(runInfo.durationMs / 1000).toFixed(1)}s</span>
            </div>
          </div>
        )}

        {/* AI Summary Card */}
        {runInfo?.aiSummary && (
          <div className="bg-gradient-to-r from-purple-50 to-blue-50 border-l-4 border-purple-600 rounded-xl p-4 border border-slate-200 shadow-xs">
            <div className="text-[11px] font-bold text-purple-600 tracking-wider mb-1.5 uppercase">🤖 AI TÓM TẮT NGUYÊN NHÂN</div>
            <div className="text-sm text-slate-700 leading-relaxed">{runInfo.aiSummary}</div>
          </div>
        )}

        {/* Mismatched / Matched Items Table */}
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={items.length}
          emptyMessage="Không có bút toán đối soát nào"
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
    default:
      return <Badge tone="success">matched</Badge>;
  }
}
