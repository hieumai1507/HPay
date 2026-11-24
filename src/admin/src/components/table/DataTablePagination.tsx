import type { Table } from '@tanstack/react-table';

interface DataTablePaginationProps<TData> {
  table: Table<TData>;
  totalCount?: number;
}

export function DataTablePagination<TData>({ table, totalCount }: DataTablePaginationProps<TData>) {
  const pageIndex = table.getState().pagination.pageIndex;
  const pageSize = table.getState().pagination.pageSize;
  const pageCount = table.getPageCount();

  const startItem = pageIndex * pageSize + 1;
  const endItem = Math.min((pageIndex + 1) * pageSize, totalCount ?? (pageIndex + 1) * pageSize);

  return (
    <div className="flex items-center justify-between px-4 py-3 bg-white border-t border-slate-200 text-xs text-slate-700">
      <div className="font-medium">
        {totalCount !== undefined ? (
          <span>
            Showing <strong>{totalCount > 0 ? startItem : 0}</strong> - <strong>{endItem}</strong> of <strong>{totalCount}</strong> results
          </span>
        ) : (
          <span>Trang <strong>{pageIndex + 1}</strong> / <strong>{Math.max(1, pageCount)}</strong></span>
        )}
      </div>

      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-slate-500">Rows per page:</span>
          <select
            value={pageSize}
            onChange={(e) => table.setPageSize(Number(e.target.value))}
            className="h-8 rounded-md border border-slate-300 px-2 text-xs bg-white cursor-pointer focus:outline-none focus:ring-1 focus:ring-brand"
          >
            {[10, 20, 50, 100].map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </div>

        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => table.setPageIndex(0)}
            disabled={!table.getCanPreviousPage()}
            className={`w-8 h-8 rounded-md border border-slate-300 bg-white flex items-center justify-center font-bold text-xs transition-colors ${
              table.getCanPreviousPage() ? 'hover:bg-slate-50 cursor-pointer text-slate-700' : 'opacity-40 cursor-not-allowed text-slate-400'
            }`}
            title="First"
          >
            ««
          </button>
          <button
            type="button"
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
            className={`w-8 h-8 rounded-md border border-slate-300 bg-white flex items-center justify-center font-bold text-xs transition-colors ${
              table.getCanPreviousPage() ? 'hover:bg-slate-50 cursor-pointer text-slate-700' : 'opacity-40 cursor-not-allowed text-slate-400'
            }`}
            title="Previous"
          >
            ‹
          </button>

          <span className="px-2 font-semibold text-xs text-slate-800">
            {pageIndex + 1} / {Math.max(1, pageCount)}
          </span>

          <button
            type="button"
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
            className={`w-8 h-8 rounded-md border border-slate-300 bg-white flex items-center justify-center font-bold text-xs transition-colors ${
              table.getCanNextPage() ? 'hover:bg-slate-50 cursor-pointer text-slate-700' : 'opacity-40 cursor-not-allowed text-slate-400'
            }`}
            title="Next"
          >
            ›
          </button>
          <button
            type="button"
            onClick={() => table.setPageIndex(pageCount - 1)}
            disabled={!table.getCanNextPage()}
            className={`w-8 h-8 rounded-md border border-slate-300 bg-white flex items-center justify-center font-bold text-xs transition-colors ${
              table.getCanNextPage() ? 'hover:bg-slate-50 cursor-pointer text-slate-700' : 'opacity-40 cursor-not-allowed text-slate-400'
            }`}
            title="Last"
          >
            »»
          </button>
        </div>
      </div>
    </div>
  );
}
