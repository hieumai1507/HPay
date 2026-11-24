import { useState, useMemo, useCallback } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useFraudFlags, useResolveFraudFlag, type FraudFlagItem } from '@/queries';
import { ResolveDialog } from '@/features/fraud/ResolveDialog';
import { Badge, ErrorNotice } from '@/ui';
import { ACTION_PERMISSION, NO_PERMISSION_REASON } from '@/lib/access';
import { formatVnd } from '@/lib/money';
import { useAdminStore } from '@/stores/admin';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const columnHelper = createColumnHelper<FraudFlagItem>();

export default function FraudPage() {
  const [statusFilter, setStatusFilter] = useState('open');
  const [ruleFilter, setRuleFilter] = useState('all');
  const [selectedFlag, setSelectedFlag] = useState<FraudFlagItem | null>(null);
  const [dialogFlag, setDialogFlag] = useState<FraudFlagItem | null>(null);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);

  const { pagination, setPagination, sorting, setSorting, queryParams } = useTableQueryState({ initialPageSize: 10 });
  // Status + paging are server-side; ruleFilter below is still client-side and
  // therefore only narrows the page currently loaded.
  const { data, isLoading, error, refetch } = useFraudFlags({
    status: statusFilter,
    page: queryParams.page,
    limit: queryParams.limit,
    order: queryParams.order as 'asc' | 'desc',
  });
  const flags = useMemo(() => data?.rows ?? [], [data]);
  const total = data?.total ?? 0;
  const resolveMutation = useResolveFraudFlag();
  // UX only: the backend does not enforce admin roles yet (audit BE-H6).
  const canResolve = useAdminStore((s) => s.can(ACTION_PERMISSION.fraud));
  const [actionError, setActionError] = useState<string | null>(null);

  const filteredFlags = useMemo(() => {
    return flags.filter((f) => {
      if (ruleFilter !== 'all' && f.rule !== ruleFilter) return false;
      return true;
    });
  }, [flags, ruleFilter]);

  const activeDetailFlag = selectedFlag || filteredFlags.find((f) => f.status === 'open') || filteredFlags[0] || null;

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id]
    );
  }, []);

  const toggleSelectAll = useCallback(() => {
    if (selectedIds.length === filteredFlags.length) {
      setSelectedIds([]);
    } else {
      setSelectedIds(filteredFlags.map((f) => f.id));
    }
  }, [filteredFlags, selectedIds.length]);

  const handleResolveConfirm = async ({ action, reason }: { action: 'clear' | 'confirm_fraud'; option: string; reason: string }) => {
    if (!dialogFlag || resolveMutation.isPending) return;
    setActionError(null);
    try {
      await resolveMutation.mutateAsync({ id: dialogFlag.id, action, reason });
      setDialogFlag(null);
      refetch();
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : String(e));
    }
  };

  const handleQuickClear = async (flag: FraudFlagItem) => {
    if (resolveMutation.isPending) return;
    if (!window.confirm(`Mark flag ${flag.flagNo} as safe?`)) return;
    setActionError(null);
    try {
      await resolveMutation.mutateAsync({ id: flag.id, action: 'clear', reason: 'Marked safe by Admin Ops' });
      refetch();
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : String(e));
    }
  };

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: 'checkbox',
        header: () => (
          <input
            type="checkbox"
            checked={selectedIds.length > 0 && selectedIds.length === filteredFlags.length}
            onChange={toggleSelectAll}
            className="rounded border-slate-300 text-brand focus:ring-brand"
          />
        ),
        cell: (info) => (
          <input
            type="checkbox"
            checked={selectedIds.includes(info.row.original.id)}
            onChange={() => toggleSelect(info.row.original.id)}
            className="rounded border-slate-300 text-brand focus:ring-brand"
          />
        ),
      }),
      columnHelper.accessor('flagNo', {
        header: 'Flag',
        cell: (info) => (
          <span
            className="font-mono font-semibold text-brand hover:underline cursor-pointer"
            onClick={() => setSelectedFlag(info.row.original)}
          >
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor('rule', {
        header: 'Rule',
        cell: (info) => <Badge tone="danger">{info.getValue()}</Badge>,
      }),
      columnHelper.accessor('score', {
        header: 'Score',
        cell: (info) => {
          const val = info.getValue();
          return (
            <b className={val >= 80 ? 'text-rose-600' : val >= 60 ? 'text-amber-600' : 'text-slate-700'}>
              {val}
            </b>
          );
        },
      }),
      columnHelper.accessor('txnRef', {
        header: 'Transaction',
        cell: (info) => <span className="font-mono">{info.getValue()}</span>,
      }),
      columnHelper.accessor('amount', {
        header: 'Amount',
        cell: (info) => <span className="font-mono font-medium">{formatVnd(info.getValue())}</span>,
      }),
      columnHelper.accessor('userName', {
        header: 'User',
        cell: (info) => <span>{info.getValue() ?? '—'}</span>,
      }),
      columnHelper.accessor('createdAt', {
        header: 'Age',
        cell: (info) => {
          const isSlaWarning = info.row.original.status === 'open' && info.row.original.score >= 80;
          return (
            <span className={isSlaWarning ? 'text-rose-600 font-bold' : 'text-slate-700 font-normal'}>
              {calculateAge(info.getValue())} {isSlaWarning ? '⚠ SLA' : ''}
            </span>
          );
        },
      }),
      columnHelper.accessor('status', {
        header: 'Status',
        cell: (info) => renderStatusBadge(info.getValue()),
      }),
    ],
    [selectedIds, filteredFlags, toggleSelect, toggleSelectAll]
  );

  const table = useReactTable({
    data: filteredFlags,
    columns,
    state: {
      pagination,
      sorting,
    },
    onPaginationChange: setPagination,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: Math.max(1, Math.ceil(total / pagination.pageSize)),
  });

  return (
    <div className="flex flex-col gap-4">
      {/* Header Bar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-bold text-slate-900">Fraud monitoring</h2>
          <span className="bg-rose-600 text-white text-xs font-bold px-2.5 py-0.5 rounded-full">
            {flags.filter((f) => f.status === 'open').length} open
          </span>
        </div>

        <div className="flex items-center gap-2.5 flex-wrap">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="h-9 px-3 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer focus:outline-none focus:ring-2 focus:ring-brand/30"
          >
            <option value="open">Status: Open</option>
            <option value="cleared">Status: Cleared</option>
            <option value="confirmed">Status: Blocked</option>
            <option value="all">Status: All</option>
          </select>

          <select
            value={ruleFilter}
            onChange={(e) => setRuleFilter(e.target.value)}
            className="h-9 px-3 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer focus:outline-none focus:ring-2 focus:ring-brand/30"
          >
            <option value="all">Rule: All</option>
            <option value="VELOCITY_5MIN">VELOCITY_5MIN</option>
            <option value="AMOUNT_SPIKE">AMOUNT_SPIKE</option>
            <option value="NEW_DEVICE">NEW_DEVICE</option>
            <option value="ODD_HOURS">ODD_HOURS</option>
          </select>

          {selectedIds.length > 0 && (
            <button
              type="button"
              disabled={!canResolve || resolveMutation.isPending}
              title={canResolve ? undefined : NO_PERMISSION_REASON}
              onClick={async () => {
                if (!window.confirm(`Mark ${selectedIds.length} flags as safe?`)) return;
                setActionError(null);
                const failed: string[] = [];
                for (const id of selectedIds) {
                  try {
                    await resolveMutation.mutateAsync({ id, action: 'clear', reason: 'Marked safe by Admin Ops' });
                  } catch {
                    failed.push(id);
                  }
                }
                setSelectedIds(failed);
                if (failed.length > 0) setActionError(`${failed.length} of the selected flags could not be resolved.`);
                refetch();
              }}
              className="h-9 px-3.5 rounded-lg border border-emerald-600 bg-emerald-50 text-xs font-semibold text-emerald-700 cursor-pointer hover:bg-emerald-100 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            >
              ☑ Mark safe ({selectedIds.length})
            </button>
          )}

          <button
            type="button"
            onClick={() => refetch()}
            className="h-9 px-3.5 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer hover:bg-slate-50 transition-colors"
          >
            ⟳ Refresh
          </button>
        </div>
      </div>

      <ErrorNotice error={error} what="fraud flags" />
      {actionError && (
        <div role="alert" className="bg-rose-50 border-l-4 border-rose-500 text-rose-700 p-3 rounded-r-lg text-sm">
          {actionError}
        </div>
      )}
      {!canResolve && <div className="text-xs text-slate-500">{NO_PERMISSION_REASON} (fraud.resolve)</div>}

      {/* Main Table */}
      <div className="flex flex-col gap-4">
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={total}
          emptyMessage="No fraud flags"
        />

        {/* Selected Flag Detail Card */}
        {activeDetailFlag && (
          <div className="bg-white rounded-xl p-4.5 border-l-4 border-l-rose-600 border border-slate-200 shadow-xs">
            <div className="text-[11px] font-bold text-slate-500 uppercase tracking-wider mb-2.5">
              FLAG {activeDetailFlag.flagNo} DETAILS — EXPLANATION
            </div>

            <div className="flex gap-6 items-center flex-wrap">
              <div className="flex-1">
                <div className="flex flex-col gap-1.5">
                  {(activeDetailFlag.rulesTriggered || []).map((r, i) => (
                    <div key={i} className="flex items-center gap-2">
                      <span className="text-rose-600 text-xs">●</span>
                      <span className="text-sm text-slate-800">{r}</span>
                    </div>
                  ))}
                  {activeDetailFlag.aiAnalysis && (
                    <div className="flex items-center gap-2 mt-1">
                      <span className="text-brand text-xs">◆</span>
                      <span className="text-sm text-slate-800">
                        <b>AI assessment:</b> {activeDetailFlag.aiAnalysis}
                      </span>
                    </div>
                  )}
                </div>
              </div>

              <div className="flex flex-col gap-2 w-72 shrink-0">
                {activeDetailFlag.status === 'open' ? (
                  <>
                    <button
                      type="button"
                      onClick={() => handleQuickClear(activeDetailFlag)}
                      disabled={!canResolve || resolveMutation.isPending}
                      title={canResolve ? undefined : NO_PERMISSION_REASON}
                      className="h-9.5 rounded-lg border border-emerald-600 bg-white text-emerald-700 font-semibold text-xs cursor-pointer flex items-center justify-center hover:bg-emerald-50 transition-colors"
                    >
                      ✓ Mark as safe
                    </button>
                    <button
                      type="button"
                      onClick={() => setDialogFlag(activeDetailFlag)}
                      disabled={!canResolve || resolveMutation.isPending}
                      title={canResolve ? undefined : NO_PERMISSION_REASON}
                      className="h-9.5 rounded-lg border-none bg-rose-600 text-white font-semibold text-xs cursor-pointer flex items-center justify-center hover:bg-rose-700 transition-colors shadow-xs"
                    >
                      🔒 Confirm fraud — choose consequence…
                    </button>
                  </>
                ) : (
                  <div className="text-xs text-slate-500 italic">
                    This flag has been handled ({activeDetailFlag.status})
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>

      <ResolveDialog
        flag={dialogFlag}
        onClose={() => setDialogFlag(null)}
        onConfirm={handleResolveConfirm}
        isSubmitting={resolveMutation.isPending}
      />
    </div>
  );
}

function calculateAge(isoString: string): string {
  const diffMinutes = Math.floor((Date.now() - new Date(isoString).getTime()) / 60000);
  if (diffMinutes < 1) return 'just now';
  if (diffMinutes < 60) return `${diffMinutes}m`;
  const hours = Math.floor(diffMinutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

function renderStatusBadge(status: FraudFlagItem['status']) {
  switch (status) {
    case 'open':
      return <Badge tone="warning">Open</Badge>;
    case 'cleared':
      return <Badge tone="success">Cleared</Badge>;
    case 'confirmed':
      return <Badge tone="danger">Blocked</Badge>;
    default:
      return <Badge tone="neutral">{status}</Badge>;
  }
}
