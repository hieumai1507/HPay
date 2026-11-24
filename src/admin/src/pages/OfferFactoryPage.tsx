import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useOffers, type OfferPackage } from '@/queries';
import { Badge, ErrorNotice } from '@/ui';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const columnHelper = createColumnHelper<OfferPackage>();

export default function OfferFactoryPage() {
  const navigate = useNavigate();
  const { pagination, setPagination, sorting, setSorting, globalFilter, setGlobalFilter, queryParams } =
    useTableQueryState({ initialPageSize: 10 });
  const { data, isLoading, error } = useOffers({
    page: queryParams.page,
    limit: queryParams.limit,
    name: globalFilter || undefined,
  });
  const offers = useMemo(() => data?.rows ?? [], [data]);
  const total = data?.total ?? 0;

  const filtered = useMemo(() => {
    if (!globalFilter) return offers;
    const q = globalFilter.toLowerCase();
    return offers.filter(
      (o) =>
        o.code.toLowerCase().includes(q) ||
        o.name.toLowerCase().includes(q)
    );
  }, [offers, globalFilter]);

  const columns = useMemo(
    () => [
      columnHelper.accessor('code', {
        header: 'Plan code',
        cell: (info) => <span className="font-mono font-semibold text-slate-900">{info.getValue()}</span>,
      }),
      columnHelper.accessor('name', {
        header: 'Plan name',
        cell: (info) => <b className="text-slate-900">{info.getValue()}</b>,
      }),
      columnHelper.accessor('role', {
        header: 'Target role',
        cell: (info) => <Badge tone={info.getValue() === 'customer' ? 'info' : 'warning'}>{info.getValue()}</Badge>,
      }),
      columnHelper.accessor('level', {
        header: 'Level',
        cell: (info) => <b>L{info.getValue()}</b>,
      }),
      columnHelper.accessor('authorizedServicesCount', {
        header: 'Authorized services',
        cell: (info) => <b>{info.getValue()}</b>,
      }),
      columnHelper.accessor('isActive', {
        header: 'Status',
        cell: (info) => (
          <Badge tone={info.getValue() ? 'success' : 'neutral'}>
            {info.getValue() ? 'Active' : 'Draft'}
          </Badge>
        ),
      }),
      columnHelper.accessor('updatedBy', {
        header: 'Updated by',
        cell: (info) => <span className="text-slate-700">{info.getValue()}</span>,
      }),
      columnHelper.display({
        id: 'action',
        header: 'Actions',
        cell: (info) => (
          <button
            type="button"
            onClick={() => navigate(`/offers/${info.row.original.id}`)}
            className="text-brand font-semibold text-xs hover:underline cursor-pointer bg-none border-none p-0"
          >
            Configure →
          </button>
        ),
      }),
    ],
    [navigate]
  );

  const table = useReactTable({
    data: filtered,
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
    manualPagination: true,
    pageCount: Math.max(1, Math.ceil(total / pagination.pageSize)),
  });

  return (
    <div className="flex flex-col gap-4">
      {/* Topbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h2 className="text-lg font-bold text-slate-900">Offer Factory (Plans &amp; Limits)</h2>

        <div className="flex items-center gap-2.5">
          <input
            type="text"
            placeholder="🔍 Search plans..."
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="h-9 w-60 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
          <button
            type="button"
            disabled
            title="Not available yet: plans cannot be created from the console. They are set up by the engineering team."
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs shadow-xs opacity-60 cursor-not-allowed"
          >
            + New plan
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-3">
        <ErrorNotice error={error} what="offers" />
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={total}
          emptyMessage="Nothing here yet"
        />

        <div className="flex items-center justify-between pt-1">
          <span className="text-xs text-slate-500">
            {offers.length} plans · Manage Limit tiers and the authentication matrix for customers and businesses.
          </span>
        </div>
      </div>
    </div>
  );
}
