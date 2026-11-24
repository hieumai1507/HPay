import { useEffect, useMemo, useState, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';

import { ApiError } from '@/api/errors';
import { toneForVerdict, useKYCQueue, type KYCQueueItem, type KYCTab } from '@/queries';
import { Badge, Input, Tabs } from '@/ui';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const TABS: { key: KYCTab; label: string }[] = [
  { key: 'manual', label: 'Pending' },
  { key: 'auto', label: 'Auto-approved by AI' },
  { key: 'rejected', label: 'Rejected' },
];

function waitLabel(createdAt: string): string {
  const mins = Math.max(0, Math.floor((Date.now() - new Date(createdAt).getTime()) / 60_000));
  if (mins < 60) return `${mins} min`;
  return `${Math.floor(mins / 60)}h${mins % 60 > 0 ? ` ${mins % 60}m` : ''}`;
}

const columnHelper = createColumnHelper<KYCQueueItem>();

export default function KYCPage() {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const tab = (params.get('tab') as KYCTab) ?? 'manual';
  const search = params.get('q') ?? '';
  const [cursor, setCursor] = useState(0);

  const { pagination, setPagination, sorting, setSorting, queryParams } = useTableQueryState({ initialPageSize: 10 });
  // tab, search and paging are all server-side now, so the queue no longer
  // loads every request just to filter it in the browser.
  const { data, isPending, error } = useKYCQueue({
    tab,
    page: queryParams.page,
    limit: queryParams.limit,
    name: search || undefined,
  });
  const rows = useMemo(() => data?.rows ?? [], [data]);
  const total = data?.total ?? 0;

  const openDetail = useCallback(
    (row: KYCQueueItem) => navigate(`/kyc/${row.id}?tab=${tab}`),
    [navigate, tab]
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement)?.tagName === 'INPUT') return;
      if (e.key === 'j' || e.key === 'J') {
        e.preventDefault();
        setCursor((c) => Math.min(c + 1, rows.length - 1));
      }
      if (e.key === 'k' || e.key === 'K') {
        e.preventDefault();
        setCursor((c) => Math.max(c - 1, 0));
      }
      if (e.key === 'Enter' && rows[cursor]) {
        e.preventDefault();
        openDetail(rows[cursor]);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [rows, cursor, openDetail]);

  const columns = useMemo(
    () => [
      columnHelper.accessor('full_name', {
        header: 'Full name',
        cell: (info) => (
          <span
            className={`cursor-pointer ${rows[cursor]?.id === info.row.original.id ? 'font-bold text-slate-900' : 'font-medium text-slate-800'}`}
            onClick={() => openDetail(info.row.original)}
          >
            {rows[cursor]?.id === info.row.original.id && <span className="text-brand">▸ </span>}
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor('phone', {
        header: 'Phone number',
        cell: (info) => <code className="font-mono text-slate-600">{info.getValue()}</code>,
      }),
      columnHelper.accessor('ai_score', {
        header: 'AI Score',
        cell: (info) => {
          const val = info.getValue();
          return (
            <span
              className={`font-bold ${
                val >= 70 ? 'text-emerald-600' : val >= 40 ? 'text-amber-600' : 'text-rose-600'
              }`}
            >
              {val}
            </span>
          );
        },
      }),
      columnHelper.accessor('ai_verdict', {
        header: 'AI Verdict',
        cell: (info) => <Badge tone={toneForVerdict(info.getValue())}>{info.getValue()}</Badge>,
      }),
      columnHelper.accessor('created_at', {
        header: 'Waiting',
        cell: (info) => waitLabel(info.getValue()),
      }),
      columnHelper.display({
        id: 'action',
        header: '',
        cell: (info) => (
          <span className="text-brand font-semibold hover:underline cursor-pointer" onClick={() => openDetail(info.row.original)}>
            Open →
          </span>
        ),
      }),
    ],
    [rows, cursor, openDetail]
  );

  const table = useReactTable({
    data: rows,
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
    <div className="flex flex-col gap-3.5">
      <div className="flex justify-between items-end gap-3 flex-wrap">
        <Tabs
          tabs={TABS.map((t) => ({ ...t, count: t.key === tab ? rows.length : undefined }))}
          active={tab}
          onChange={(k) => {
            setCursor(0);
            setParams((p) => {
              p.set('tab', k);
              return p;
            });
          }}
        />
        <div className="w-64">
          <Input
            placeholder="Search name / phone number..."
            value={search}
            onChange={(e) =>
              setParams((p) => {
                if (e.target.value) p.set('q', e.target.value);
                else p.delete('q');
                return p;
              })
            }
          />
        </div>
      </div>

      <div className="text-xs text-slate-500">
        Shortcuts: <code className="font-mono bg-slate-100 px-1 rounded">J</code>/<code className="font-mono bg-slate-100 px-1 rounded">K</code> move · <code className="font-mono bg-slate-100 px-1 rounded">Enter</code> open request
      </div>

      {error ? (
        <div className="text-rose-600 p-4 bg-rose-50 rounded-lg border border-rose-200">
          {error instanceof ApiError ? error.message : 'Something went wrong. Please try again.'}
        </div>
      ) : (
        <DataTable table={table} isLoading={isPending} totalCount={total} emptyMessage="Nothing here yet" />
      )}
    </div>
  );
}
