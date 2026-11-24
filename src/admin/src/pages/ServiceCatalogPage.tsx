import { useState, useMemo, useCallback } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  createColumnHelper,
} from '@tanstack/react-table';
import { useServices, useUpdateServiceStatus, type ServiceItem } from '@/queries';
import { ACTION_PERMISSION, NO_PERMISSION_REASON } from '@/lib/access';
import { useAdminStore } from '@/stores/admin';
import { ServicePipelineModal } from '@/features/services/ServicePipelineModal';
import { Badge, ErrorNotice, Modal, NotAvailableNote } from '@/ui';
import { DataTable } from '@/components/table/DataTable';
import { useTableQueryState } from '@/hooks/useTableQueryState';

const columnHelper = createColumnHelper<ServiceItem>();

export default function ServiceCatalogPage() {

  const updateStatusMutation = useUpdateServiceStatus();
  // UX only: the backend does not enforce admin roles yet (audit BE-H6).
  const canConfigure = useAdminStore((s) => s.can(ACTION_PERMISSION.serviceConfig));
  const [statusError, setStatusError] = useState<string | null>(null);

  const [openAddModal, setOpenAddModal] = useState(false);
  const [selectedService, setSelectedService] = useState<ServiceItem | null>(null);

  const { pagination, setPagination, sorting, setSorting, globalFilter, setGlobalFilter, queryParams } =
    useTableQueryState({ initialPageSize: 10 });
  // Search is server-side (services.name/code ILIKE), so `filtered` below is
  // just the rows this page returned.
  const { data, isLoading, error } = useServices({
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
          `Confirm changing the status of service ${service.name} to: ${newStatus.toUpperCase()}?`,
        )
      )
        return;
      if (updateStatusMutation.isPending) return;
      setStatusError(null);
      try {
        await updateStatusMutation.mutateAsync({ id: service.id, status: newStatus });
      } catch (e: unknown) {
        setStatusError(e instanceof Error ? e.message : String(e));
      }
    },
    [updateStatusMutation]
  );

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    // Backend gap: there is no POST /v1/admin/services route, so nothing can be created.
    // The submit button is disabled; this guard keeps Enter-key submits from doing anything.
    return;
  };

  const columns = useMemo(
    () => [
      columnHelper.accessor('code', {
        header: 'Service code',
        cell: (info) => <span className="font-mono font-bold">{info.getValue()}</span>,
      }),
      columnHelper.accessor('name', {
        header: 'Service name / Full name',
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
        header: 'Transaction type',
        cell: (info) => (
          <Badge tone={info.getValue() === 'Refund' ? 'warning' : 'neutral'}>
            {info.getValue() || 'Normal Transaction'}
          </Badge>
        ),
      }),
      columnHelper.accessor('actionPriority', {
        header: 'Action priority',
        cell: (info) => (
          <span className="text-xs font-semibold text-slate-700">
            {info.getValue() === 'before_gl' ? 'Before (3rd party → GL)' : 'After (GL → 3rd party)'}
          </span>
        ),
      }),
      columnHelper.accessor('status', {
        header: 'Status',
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
        header: 'Actions',
        cell: (info) => (
          <button
            type="button"
            onClick={() => handleToggleMaintenance(info.row.original)}
            disabled={!canConfigure || updateStatusMutation.isPending}
            title={canConfigure ? undefined : NO_PERMISSION_REASON}
            className={`font-semibold cursor-pointer text-xs ${
              info.row.original.status === 'maintenance' ? 'text-emerald-600 hover:underline' : 'text-amber-600 hover:underline'
            }`}
          >
            {info.row.original.status === 'maintenance' ? 'Reactivate →' : 'Put in maintenance ⚡'}
          </button>
        ),
      }),
    ],
    [handleToggleMaintenance, canConfigure, updateStatusMutation.isPending]
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
        <h2 className="text-lg font-bold text-slate-900">Service catalog (Low-Code Configurator)</h2>

        <div className="flex items-center gap-2.5">
          <input
            type="text"
            placeholder="🔍 Search by code or name..."
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="h-9 w-60 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
          <button
            type="button"
            disabled
            title="Not available yet: the backend has no create-service endpoint (POST /v1/admin/services)."
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 transition-colors shadow-xs"
          >
            + Add service
          </button>
        </div>
      </div>

      <ErrorNotice error={error} what="services" />
      {statusError && (
        <div role="alert" className="bg-rose-50 border-l-4 border-rose-500 text-rose-700 p-3 rounded-r-lg text-sm">
          {statusError}
        </div>
      )}
      <NotAvailableNote>{'creating services (no POST /v1/admin/services) and saving pipeline designs (no endpoint) are not supported by the backend.'}</NotAvailableNote>

      <div className="flex flex-col gap-3">
        <DataTable
          table={table}
          isLoading={isLoading}
          totalCount={total}
          emptyMessage="Nothing here yet"
        />

        <div className="flex items-center justify-between pt-1">
          <span className="text-xs text-slate-500">
            {services.length} system services · Supports setting up the Action Pipeline (GL Posting / Directional Integration / Internal Ingression) &amp; input validation rules.
          </span>
        </div>
      </div>

      {/* Add Modal */}
      <Modal open={openAddModal} onClose={() => setOpenAddModal(false)} title="Add a service to the catalog">
        <form onSubmit={handleCreate} className="flex flex-col gap-3">
          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Service code</label>
              <input
                type="text"
                placeholder="E.g. P2P_TRANSFER, BILL_PAYMENT"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                required
              />
            </div>
            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Service name</label>
              <input
                type="text"
                placeholder="E.g. P2P Transfer"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                required
              />
            </div>
          </div>

          <div>
            <label className="text-xs font-bold text-slate-500 mb-1 block">Full name</label>
            <input
              type="text"
              placeholder="E.g. Subscriber Transfer To Wallet (P2P)"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
          </div>

          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Category</label>
              <select
                value={category}
                onChange={(e) => setCategory(e.target.value as ServiceItem['category'])}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
              >
                <option value="transfer">transfer (Transfer)</option>
                <option value="payment">payment (Payment)</option>
                <option value="wallet">wallet (Wallet, top up &amp; withdraw)</option>
                <option value="org">org (Organization)</option>
              </select>
            </div>

            <div>
              <label className="text-xs font-bold text-slate-500 mb-1 block">Transaction type</label>
              <select
                value={transactionType}
                onChange={(e) => setTransactionType(e.target.value)}
                className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
              >
                <option value="Normal Transaction">Normal Transaction</option>
                <option value="Refund">Refund</option>
                <option value="Inquiry">Inquiry</option>
              </select>
            </div>
          </div>

          <div>
            <label className="text-xs font-bold text-slate-500 mb-1 block">Action priority</label>
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
              Cancel
            </button>
            <button
              type="submit"
              disabled
              title="Not available yet: the backend has no create-service endpoint."
              className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
            >
              Confirm create service
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
      return <Badge tone="warning">Maintenance</Badge>;
    case 'disabled':
    default:
      return <Badge tone="neutral">Disabled</Badge>;
  }
}
