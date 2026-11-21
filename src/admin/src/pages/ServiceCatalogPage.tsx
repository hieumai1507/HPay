import { useState, useMemo, useCallback } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useServices, useUpdateServiceStatus, useCreateService, type ServiceItem } from '@/queries';
import { ServicePipelineModal } from '@/features/services/ServicePipelineModal';
import { Badge, Modal } from '@/ui';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const columnHelper = createColumnHelper<ServiceItem>();

export default function ServiceCatalogPage() {

  const updateStatusMutation = useUpdateServiceStatus();
  const createServiceMutation = useCreateService();

  const [openAddModal, setOpenAddModal] = useState(false);
  const [selectedService, setSelectedService] = useState<ServiceItem | null>(null);

  const { pagination, setPagination, sorting, setSorting, globalFilter, setGlobalFilter, queryParams } =
    useTableQueryState({ initialPageSize: 10 });
  // Search is server-side (services.name/code ILIKE), so `filtered` below is
  // just the rows this page returned.
  const { data, isLoading } = useServices({
    page: queryParams.page,
    limit: queryParams.limit,
    name: globalFilter || undefined,
  });
  const services = data?.rows ?? [];
  const total = data?.total ?? 0;

  // Form states
  const [code, setCode] = useState('');
  const [name, setName] = useState('');
  const [fullName, setFullName] = useState('');
  const [category, setCategory] = useState<ServiceItem['category']>('transfer');
  const [transactionType, setTransactionType] = useState('Normal Transaction');
  const [actionPriority, setActionPriority] = useState<ServiceItem['actionPriority']>('before_gl');

  const filtered = services;

  const handleToggleMaintenance = useCallback(
    async (service: ServiceItem) => {
      const newStatus: ServiceItem['status'] = service.status === 'maintenance' ? 'active' : 'maintenance';
      if (
        !window.confirm(
          `Xác nhận chuyển trạng thái dịch vụ ${service.name} sang: ${newStatus.toUpperCase()}?`,
        )
      )
        return;
      await updateStatusMutation.mutateAsync({ id: service.id, status: newStatus });
    },
    [updateStatusMutation]
  );

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!code.trim() || !name.trim()) {
      alert('Vui lòng nhập mã và tên dịch vụ.');
      return;
    }
    await createServiceMutation.mutateAsync({
      code: code.trim().toUpperCase(),
      name: name.trim(),
      fullName: fullName.trim() || name.trim(),
      category,
      transactionType,
      actionPriority,
      status: 'active',
      channels: ['Mobile'],
    });
    setOpenAddModal(false);
    setCode('');
    setName('');
    setFullName('');
    alert('Đã tạo Dịch vụ mới thành công!');
  };

  const columns = useMemo(
    () => [
      columnHelper.accessor('code', {
        header: 'Mã Dịch vụ',
        cell: (info) => <span className="font-mono font-bold">{info.getValue()}</span>,
      }),
      columnHelper.accessor('name', {
        header: 'Tên Dịch vụ / Tên đầy đủ',
        cell: (info) => (
          <div>
            <b className="text-slate-900">{info.getValue()}</b>
            {info.row.original.fullName && info.row.original.fullName !== info.getValue() && (
              <div className="text-[11.5px] text-slate-500">{info.row.original.fullName}</div>
            )}
          </div>
        ),
      }),
      columnHelper.accessor('category', {
        header: 'Category',
        cell: (info) => <Badge tone="info">{info.getValue()}</Badge>,
      }),
      columnHelper.accessor('transactionType', {
        header: 'Loại Giao dịch',
        cell: (info) => (
          <Badge tone={info.getValue() === 'Refund' ? 'warning' : 'neutral'}>
            {info.getValue() || 'Normal Transaction'}
          </Badge>
        ),
      }),
      columnHelper.accessor('actionPriority', {
        header: 'Ưu tiên Action',
        cell: (info) => (
          <span className="text-xs font-semibold text-slate-700">
            {info.getValue() === 'before_gl' ? 'Before (3rd party → GL)' : 'After (GL → 3rd party)'}
          </span>
        ),
      }),
      columnHelper.accessor('status', {
        header: 'Trạng thái',
        cell: (info) => renderStatusBadge(info.getValue()),
      }),
      columnHelper.display({
        id: 'actionsPipeline',
        header: 'Action Pipeline',
        cell: (info) => (
          <button
            type="button"
            onClick={() => setSelectedService(info.row.original)}
            className="px-2.5 py-1 rounded-md border border-brand bg-cyan-50 text-brand text-xs font-semibold hover:bg-cyan-100 transition-colors cursor-pointer"
          >
            ⚡ Low-Code Pipeline ({info.row.original.actions?.length || 2} steps)
          </button>
        ),
      }),
      columnHelper.display({
        id: 'action',
        header: 'Thao tác',
        cell: (info) => (
          <button
            type="button"
            onClick={() => handleToggleMaintenance(info.row.original)}
            className={`font-semibold cursor-pointer text-xs ${
              info.row.original.status === 'maintenance' ? 'text-emerald-600 hover:underline' : 'text-amber-600 hover:underline'
            }`}
          >
            {info.row.original.status === 'maintenance' ? 'Kích hoạt lại →' : 'Tạm bảo trì ⚡'}
          </button>
        ),
      }),
    ],
    [handleToggleMaintenance]
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
        <h2 className="text-lg font-bold text-slate-900">Danh mục Dịch vụ (Service Catalog &amp; Low-Code Configurator)</h2>

        <div className="flex items-center gap-2.5">
          <input
            type="text"
            placeholder="🔍 Tìm mã, tên dịch vụ…"
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="h-9 w-60 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
          <button
            type="button"
            onClick={() => setOpenAddModal(true)}
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 transition-colors shadow-xs"
          >
            + Thêm dịch vụ mới
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-3">
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={total}
          emptyMessage="Không tìm thấy dịch vụ nào trong Catalog"
        />

        <div className="flex items-center justify-between pt-1">
          <span className="text-xs text-slate-500">
            {services.length} Dịch vụ hệ thống · Hỗ trợ thiết lập Action Pipeline (GL Posting / Directional Integration / Internal Ingression) &amp; Input Validation Rules.
          </span>
        </div>
      </div>

      {/* Add Modal */}
      <Modal open={openAddModal} onClose={() => setOpenAddModal(false)} title="Thêm Dịch vụ mới vào Catalog">
        <form onSubmit={handleCreate} className="flex flex-col gap-3">
          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Mã Dịch vụ (Code)</label>
              <input
                type="text"
                placeholder="Vd: P2P_TRANSFER, BILL_PAYMENT"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                required
              />
            </div>
            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Tên Dịch vụ</label>
              <input
                type="text"
                placeholder="Vd: Chuyển tiền P2P"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                required
              />
            </div>
          </div>

          <div>
            <label className="text-xs font-bold text-slate-500 mb-1 block">Tên đầy đủ (Full Name)</label>
            <input
              type="text"
              placeholder="Vd: Subscriber Transfer To Wallet (P2P)"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
          </div>

          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Phân loại Category</label>
              <select
                value={category}
                onChange={(e) => setCategory(e.target.value as ServiceItem['category'])}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
              >
                <option value="transfer">transfer (Chuyển tiền)</option>
                <option value="payment">payment (Thanh toán)</option>
                <option value="wallet">wallet (Ví &amp; Nạp rút)</option>
                <option value="org">org (Doanh nghiệp)</option>
              </select>
            </div>

            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Loại Giao dịch (Transaction Type)</label>
              <select
                value={transactionType}
                onChange={(e) => setTransactionType(e.target.value)}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
              >
                <option value="Normal Transaction">Normal Transaction</option>
                <option value="Refund">Refund (Hoàn tiền)</option>
                <option value="Inquiry">Inquiry (Tra cứu)</option>
              </select>
            </div>
          </div>

          <div>
            <label className="text-xs font-bold text-slate-500 mb-1 block">Thứ tự Ưu tiên (Action Priority)</label>
            <select
              value={actionPriority}
              onChange={(e) => setActionPriority(e.target.value as ServiceItem['actionPriority'])}
              className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
            >
              <option value="before_gl">Before: Call 3rd party first then Post to GL</option>
              <option value="after_gl">After: Post to GL first then call 3rd party</option>
            </select>
          </div>

          <div className="flex justify-end gap-2.5 mt-2">
            <button
              type="button"
              onClick={() => setOpenAddModal(false)}
              className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50"
            >
              Hủy
            </button>
            <button
              type="submit"
              disabled={createServiceMutation.isPending}
              className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
            >
              {createServiceMutation.isPending ? 'Đang tạo…' : 'Xác nhận tạo Dịch vụ'}
            </button>
          </div>
        </form>
      </Modal>

      {/* Action Pipeline Configurator Modal */}
      <ServicePipelineModal
        open={!!selectedService}
        onClose={() => setSelectedService(null)}
        service={selectedService}
      />
    </div>
  );
}

function renderStatusBadge(status: ServiceItem['status']) {
  switch (status) {
    case 'active':
      return <Badge tone="success">Active</Badge>;
    case 'maintenance':
      return <Badge tone="warning">Bảo trì (Maintenance)</Badge>;
    case 'disabled':
    default:
      return <Badge tone="neutral">Disabled</Badge>;
  }
}
