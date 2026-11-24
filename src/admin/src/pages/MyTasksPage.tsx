import { useMemo, useState } from 'react';
import { useReactTable, getCoreRowModel, createColumnHelper } from '@tanstack/react-table';

import { ApiError } from '@/api/errors';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';
import {
  useAdminApprovals,
  useDecideApproval,
  type AdminApprovalItem,
} from '@/queries/useApprovalQueries';
import { formatVnd } from '@/lib/money';
import { useAdminStore } from '@/stores/admin';
import { useToastStore } from '@/stores/toast';
import { Badge, ErrorNotice } from '@/ui';

const columnHelper = createColumnHelper<AdminApprovalItem>();

const MIN_REASON = 10;

function subjectTone(subject: string): 'danger' | 'warning' | 'info' {
  if (subject === 'reversal') return 'danger';
  if (subject === 'account_lock') return 'warning';
  return 'info';
}

function formatTaskAmount(v: number) {
  return v ? formatVnd(v) : '—';
}

export default function MyTasksPage() {
  const { pagination, setPagination, sorting, setSorting, queryParams } = useTableQueryState({
    initialPageSize: 10,
  });
  const can = useAdminStore((s) => s.can);
  const toast = useToastStore((s) => s.push);

  // Reversal / account_lock / policy_change are ops-grade actions; the backend
  // re-checks the admin token regardless of what the UI chooses to show.
  const canDecide = can('workflow.action');

  const serverParams = useMemo(
    () => ({ page: queryParams.page, limit: queryParams.limit }),
    [queryParams.page, queryParams.limit]
  );

  const { data, isLoading, error } = useAdminApprovals(serverParams);
  const rows = data?.rows ?? [];
  const total = data?.total ?? 0;

  const decide = useDecideApproval();
  const [selected, setSelected] = useState<AdminApprovalItem | null>(null);
  const [reason, setReason] = useState('');

  function close() {
    setSelected(null);
    setReason('');
  }

  function submit(action: 'approve' | 'reject') {
    if (!selected) return;
    if (reason.trim().length < MIN_REASON) {
      toast(`Reason must be at least ${MIN_REASON} characters (recorded in the audit log).`, 'error');
      return;
    }
    decide.mutate(
      { id: selected.id, action, reason: reason.trim() },
      {
        onSuccess: () => {
          toast(action === 'approve' ? 'Order approved.' : 'Order rejected.', 'success');
          close();
        },
        onError: (err) => {
          if (err instanceof ApiError && err.status === 409) {
            toast('This order was handled elsewhere — the list has been updated.', 'info');
            close();
            return;
          }
          toast(err instanceof ApiError ? err.message : 'Something went wrong. Please try again.', 'error');
        },
      }
    );
  }

  const columns = useMemo(
    () => [
      columnHelper.accessor('subjectType', {
        header: 'Order type',
        cell: (info) => (
          <Badge tone={subjectTone(info.getValue())}>{info.getValue()}</Badge>
        ),
      }),
      columnHelper.accessor('refCode', {
        header: 'Transaction code',
        cell: (info) => (
          <span className="font-mono text-slate-600">{info.getValue() || '—'}</span>
        ),
      }),
      columnHelper.accessor('amount', {
        header: 'Amount',
        cell: (info) => (
          <span className="tabular-nums font-semibold">{formatTaskAmount(info.getValue())}</span>
        ),
      }),
      columnHelper.accessor('reason', {
        header: 'Request reason',
        cell: (info) => (
          <span className="text-slate-700">{info.getValue() || 'No note'}</span>
        ),
      }),
      columnHelper.accessor('currentLevel', {
        header: 'Approval level',
        cell: (info) => `L${info.getValue()}`,
      }),
      columnHelper.accessor('expiresAt', {
        header: 'Expires',
        cell: (info) => (
          <span className="text-slate-500">
            {new Date(info.getValue()).toLocaleString('en-GB')}
          </span>
        ),
      }),
      columnHelper.display({
        id: 'actions',
        header: '',
        cell: (ctx) =>
          canDecide ? (
            <button
              type="button"
              onClick={() => {
                setSelected(ctx.row.original);
                setReason('');
              }}
              className="px-3 py-1.5 rounded-lg bg-brand text-white text-xs font-bold hover:opacity-90 cursor-pointer"
            >
              Process
            </button>
          ) : (
            <span className="text-xs text-slate-400">View only</span>
          ),
      }),
    ],
    [canDecide]
  );

  const table = useReactTable({
    data: rows,
    columns,
    state: { pagination, sorting },
    onPaginationChange: setPagination,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: Math.max(1, Math.ceil(total / pagination.pageSize)),
  });

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">My Tasks — Approval inbox</h1>
        <p className="mt-1 text-sm text-slate-500">
          Admin orders awaiting your decision: reversals, account locks, policy changes.
          Every decision is recorded with its reason in the audit log.
        </p>
      </div>

      <ErrorNotice error={error} what="approvals" />
      <DataTable
        table={table}
        isLoading={isLoading}
        totalCount={total}
        emptyMessage="No orders awaiting approval 🎉"
      />

      {selected && (
        <div className="fixed inset-0 bg-slate-900/40 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg p-6 flex flex-col gap-4">
            <div>
              <h2 className="text-lg font-bold text-slate-900">
                Process order <Badge tone={subjectTone(selected.subjectType)}>{selected.subjectType}</Badge>
              </h2>
              <p className="mt-1 text-sm text-slate-500">
                {selected.refCode ? `Transaction ${selected.refCode} · ` : ''}
                {formatTaskAmount(selected.amount)}
              </p>
            </div>

            <label className="flex flex-col gap-1.5 text-sm">
              <span className="font-semibold text-slate-700">
                Decision reason <span className="text-rose-600">*</span>
              </span>
              <textarea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={3}
                placeholder={`At least ${MIN_REASON} characters — recorded in the audit log`}
                className="border border-slate-300 rounded-lg px-3 py-2 text-sm focus:outline-hidden focus:ring-2 focus:ring-brand/40"
              />
              <span className="text-xs text-slate-400">{reason.trim().length}/{MIN_REASON}</span>
            </label>

            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={close}
                disabled={decide.isPending}
                className="px-4 py-2 rounded-lg border border-slate-300 text-sm font-semibold text-slate-700 hover:bg-slate-50 cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => submit('reject')}
                disabled={decide.isPending}
                className="px-4 py-2 rounded-lg bg-rose-600 text-white text-sm font-bold hover:opacity-90 disabled:opacity-50 cursor-pointer"
              >
                Reject
              </button>
              <button
                type="button"
                onClick={() => submit('approve')}
                disabled={decide.isPending}
                className="px-4 py-2 rounded-lg bg-emerald-600 text-white text-sm font-bold hover:opacity-90 disabled:opacity-50 cursor-pointer"
              >
                Approve
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
