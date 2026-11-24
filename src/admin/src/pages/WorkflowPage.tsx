import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useWorkflowInstances, type WorkflowInstance } from '@/queries';
import { Badge, ErrorNotice } from '@/ui';
import { formatUsd } from '@/lib/money';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const columnHelper = createColumnHelper<WorkflowInstance>();

export default function WorkflowPage() {
  const navigate = useNavigate();
  const [stuckOnly, setStuckOnly] = useState(false);
  const [statusFilter, setStatusFilter] = useState('all');
  const [defFilter, setDefFilter] = useState('all');

  const { pagination, setPagination, sorting, setSorting, queryParams } = useTableQueryState({ initialPageSize: 10 });

  // stuckOnly/status/definition stay client-side (no API filter yet) — the hook
  // reports the narrowed count in that case so pageCount never over-promises.
  const { data, isLoading, error, refetch } = useWorkflowInstances({
    stuckOnly,
    status: statusFilter,
    definition: defFilter,
    page: queryParams.page,
    limit: queryParams.limit,
  });
  const instances = data?.rows ?? [];
  const total = data?.total ?? 0;

  const stuckCount = instances.filter((i) => i.isStuck).length;

  const columns = useMemo(
    () => [
      columnHelper.accessor('id', {
        header: 'Instance',
        cell: (info) => <span className="font-mono">{info.getValue()}</span>,
      }),
      columnHelper.accessor('definitionId', {
        header: 'Definition',
        cell: (info) => <span className="text-slate-800">{info.getValue()}</span>,
      }),
      columnHelper.accessor('txnRef', {
        header: 'Transaction',
        cell: (info) => <span className="font-mono">{info.getValue() || '—'}</span>,
      }),
      columnHelper.accessor('amount', {
        header: 'Amount',
        cell: (info) => (
          <span className="font-mono font-semibold">
            {formatUsd(info.getValue())}
          </span>
        ),
      }),
      columnHelper.accessor('currentStep', {
        header: 'Current step',
        cell: (info) => <b>{info.getValue()}</b>,
      }),
      columnHelper.accessor('age', {
        header: 'Age',
        cell: (info) => (
          <span className={info.row.original.isStuck ? 'text-rose-600 font-bold' : 'text-slate-700'}>
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor('status', {
        header: 'Status',
        cell: (info) => renderStatusBadge(info.getValue()),
      }),
      columnHelper.display({
        id: 'action',
        header: '',
        cell: (info) => (
          <button
            type="button"
            onClick={() => navigate(`/workflows/${info.row.original.id}`)}
            className="text-brand font-semibold text-xs hover:underline cursor-pointer bg-none border-none p-0"
          >
            Open →
          </button>
        ),
      }),
    ],
    [navigate]
  );

  const table = useReactTable({
    data: instances,
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
      {/* Topbar / Toolbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-bold text-slate-900">Workflow Monitor</h2>
          {stuckCount > 0 && (
            <span className="bg-rose-600 text-white text-xs font-bold px-2.5 py-0.5 rounded-full">
              {stuckCount} stuck &gt; 5m
            </span>
          )}
        </div>

        <div className="flex items-center gap-2.5 flex-wrap">
          {/* Stuck filter toggle */}
          <button
            type="button"
            onClick={() => setStuckOnly(!stuckOnly)}
            className={`h-9 px-3.5 rounded-lg border text-xs font-medium cursor-pointer transition-colors ${
              stuckOnly ? 'border-rose-600 bg-rose-50 text-rose-600' : 'border-slate-200 bg-white text-slate-700'
            }`}
          >
            Stuck &gt; 5 min
          </button>

          {/* Status filter */}
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="h-9 px-3 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer focus:outline-none focus:ring-2 focus:ring-brand/30"
          >
            <option value="all">Status: All</option>
            <option value="failed">Failed</option>
            <option value="running">Running</option>
            <option value="waiting">Waiting approval</option>
            <option value="completed">Completed</option>
            <option value="compensated">Compensated</option>
          </select>

          {/* Definition filter */}
          <select
            value={defFilter}
            onChange={(e) => setDefFilter(e.target.value)}
            className="h-9 px-3 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer focus:outline-none focus:ring-2 focus:ring-brand/30"
          >
            <option value="all">Definition: All</option>
            <option value="org_disbursement">org_disbursement</option>
            <option value="p2p_transfer">p2p_transfer</option>
            <option value="kyc_verification">kyc_verification</option>
          </select>

          <button
            type="button"
            onClick={() => refetch()}
            className="h-9 px-3.5 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer hover:bg-slate-50 transition-colors"
          >
            ⟳ Refresh
          </button>
        </div>
      </div>

      {/* Page Content */}
      <ErrorNotice error={error} what="workflows" />
      <div className="flex flex-col gap-3">
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={total}
          emptyMessage="Nothing here yet"
        />

        {/* Footer info */}
        <div className="flex items-center justify-between pt-1">
          <span className="text-xs text-slate-500">{instances.length} instances now · auto-refresh 10s</span>
        </div>
      </div>
    </div>
  );
}

function renderStatusBadge(status: WorkflowInstance['status']) {
  switch (status) {
    case 'failed':
      return <Badge tone="danger">Failed</Badge>;
    case 'running':
      return <Badge tone="info">Running</Badge>;
    case 'waiting':
    case 'waiting_approval':
      return <Badge tone="warning">Waiting approval</Badge>;
    case 'recovering':
      return <Badge tone="info">Recovering</Badge>;
    case 'compensating':
      return <Badge tone="warning">Compensating</Badge>;
    case 'compensation_failed':
      return <Badge tone="danger">Compensation failed</Badge>;
    case 'completed':
      return <Badge tone="success">Completed</Badge>;
    case 'compensated':
      return <Badge tone="neutral">Compensated ↩</Badge>;
    default:
      return <Badge tone="neutral">{status}</Badge>;
  }
}
