import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useOffers, type OfferPackage } from '@/queries';
import { Badge } from '@/ui';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const columnHelper = createColumnHelper<OfferPackage>();

export default function OfferFactoryPage() {
  const navigate = useNavigate();
  const { pagination, setPagination, sorting, setSorting, globalFilter, setGlobalFilter, queryParams } =
    useTableQueryState({ initialPageSize: 10 });
  const { data, isLoading } = useOffers({
    page: queryParams.page,
    limit: queryParams.limit,
    name: globalFilter || undefined,
  });
  const offers = data?.rows ?? [];
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
        header: 'Mã Offer',
        cell: (info) => <span className="font-mono font-semibold text-slate-900">{info.getValue()}</span>,
      }),
      columnHelper.accessor('name', {
        header: 'Tên Gói Sản phẩm',
        cell: (info) => <b className="text-slate-900">{info.getValue()}</b>,
      }),
      columnHelper.accessor('role', {
        header: 'Role Đối tượng',
        cell: (info) => <Badge tone={info.getValue() === 'customer' ? 'info' : 'warning'}>{info.getValue()}</Badge>,
      }),
      columnHelper.accessor('level', {
        header: 'Level',
        cell: (info) => <b>L{info.getValue()}</b>,
      }),
      columnHelper.accessor('authorizedServicesCount', {
        header: 'Dịch vụ Cấp phép',
        cell: (info) => <b>{info.getValue()}</b>,
      }),
      columnHelper.accessor('isActive', {
        header: 'Trạng thái',
        cell: (info) => (
          <Badge tone={info.getValue() ? 'success' : 'neutral'}>
            {info.getValue() ? 'Active' : 'Draft'}
          </Badge>
        ),
      }),
      columnHelper.accessor('updatedBy', {
        header: 'Cập nhật bởi',
        cell: (info) => <span className="text-slate-700">{info.getValue()}</span>,
      }),
      columnHelper.display({
        id: 'action',
        header: 'Thao tác',
        cell: (info) => (
          <button
            type="button"
            onClick={() => navigate(`/offers/${info.row.original.id}`)}
            className="text-brand font-semibold text-xs hover:underline cursor-pointer bg-none border-none p-0"
          >
            Cấu hình →
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
        <h2 className="text-lg font-bold text-slate-900">Offer Factory (Gói sản phẩm &amp; Hạn mức)</h2>

        <div className="flex items-center gap-2.5">
          <input
            type="text"
            placeholder="🔍 Tìm gói Offer…"
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="h-9 w-60 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
          <button
            type="button"
            onClick={() => navigate('/offers/new')}
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 transition-colors shadow-xs"
          >
            + Tạo Offer mới
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-3">
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={total}
          emptyMessage="Không tìm thấy gói sản phẩm nào"
        />

        <div className="flex items-center justify-between pt-1">
          <span className="text-xs text-slate-500">
            {offers.length} Gói Offer · Cho phép quản lý phân hạng hạn mức &amp; ma trận xác thực cho Khách hàng &amp; Doanh nghiệp.
          </span>
        </div>
      </div>
    </div>
  );
}
