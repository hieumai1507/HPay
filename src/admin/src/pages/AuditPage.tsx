import { useMemo } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useAuditQueries, type AuditLogItem } from '@/queries';
import { Badge, ErrorNotice } from '@/ui';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const columnHelper = createColumnHelper<AuditLogItem>();

export default function AuditPage() {
  const { pagination, setPagination, sorting, setSorting, globalFilter, setGlobalFilter, queryParams } =
    useTableQueryState({ initialPageSize: 10 });

  // Server-side pagination + search (docs plan audit-log-search):
  // /v1/admin/audit now matches `name` against action/resource/reason.
  const serverParams = useMemo(
    () => ({
      page: queryParams.page,
      limit: queryParams.limit,
      sort: queryParams.sort,
      order: queryParams.order as 'asc' | 'desc',
      name: globalFilter || undefined,
    }),
    [queryParams.page, queryParams.limit, queryParams.sort, queryParams.order, globalFilter]
  );

  const { data, isLoading, error } = useAuditQueries(serverParams);
  const logs = data?.rows ?? [];
  const total = data?.total ?? 0;

  const columns = useMemo(
    () => [
      columnHelper.accessor('timestamp', {
        header: 'Thời điểm',
        cell: (info) => <span className="font-mono">{info.getValue()}</span>,
      }),
      columnHelper.accessor('actor', {
        header: 'Actor',
        cell: (info) => (
          <div>
            <div className="font-semibold text-slate-900">{info.getValue()}</div>
            <div className="text-xs text-slate-500">{info.row.original.actorRole}</div>
          </div>
        ),
      }),
      columnHelper.accessor('action', {
        header: 'Hành động (Action)',
        cell: (info) => <Badge tone={info.row.original.actionType}>{info.getValue()}</Badge>,
      }),
      columnHelper.accessor('resource', {
        header: 'Tài nguyên',
        cell: (info) => <span className="font-semibold text-slate-800">{info.getValue()}</span>,
      }),
      columnHelper.accessor('details', {
        header: 'Chi tiết sự kiện',
        cell: (info) => <span className="text-slate-700">{info.getValue()}</span>,
      }),
      columnHelper.accessor('ip', {
        header: 'IP Source',
        cell: (info) => <span className="font-mono text-slate-600">{info.getValue()}</span>,
      }),
    ],
    []
  );

  const table = useReactTable({
    data: logs,
    columns,
    state: {
      pagination,
      sorting,
      globalFilter,
    },
    onPaginationChange: setPagination,
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    // The server already sliced the page, so the table must not slice again.
    manualPagination: true,
    manualSorting: true,
    pageCount: Math.max(1, Math.ceil(total / pagination.pageSize)),
  });

  return (
    <div className="flex flex-col gap-5">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">
            Audit Logs &amp; System Trails
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            Nhật ký kiểm toán thời gian thực theo dõi từng hành động admin &amp; sự kiện giao dịch.
          </p>
        </div>
      </div>

      {/* Toolbar Search */}
      <div className="flex gap-3 items-center">
        <input
          type="text"
          placeholder="🔍 Tìm theo Action, Resource, lý do..."
          value={globalFilter}
          onChange={(e) => setGlobalFilter(e.target.value)}
          className="h-9 w-80 rounded-lg border border-slate-200 px-3 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-brand/30 focus:border-brand"
        />
      </div>

      <ErrorNotice error={error} what="audit log" />

      {/* TanStack Table Integration */}
      <DataTable table={table} isLoading={isLoading} totalCount={total} />
    </div>
  );
}
