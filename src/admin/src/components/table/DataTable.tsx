import { flexRender, type Table as TanStackTable } from '@tanstack/react-table';
import { DataTablePagination } from './DataTablePagination';

interface DataTableProps<TData> {
  table: TanStackTable<TData>;
  isLoading?: boolean;
  totalCount?: number;
  emptyMessage?: string;
  /** Makes the whole row clickable. Buttons and links inside a row keep their own action. */
  onRowClick?: (row: TData) => void;
}

export function DataTable<TData>({
  table,
  isLoading = false,
  totalCount,
  emptyMessage = 'Nothing here yet',
  onRowClick,
}: DataTableProps<TData>) {
  return (
    <div className="bg-white rounded-xl border border-slate-200 overflow-hidden shadow-xs">
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-xs text-left">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id} className="border-b border-slate-200 bg-slate-50">
                {headerGroup.headers.map((header) => (
                  <th
                    key={header.id}
                    className={`px-4 py-3 font-bold text-slate-600 select-none whitespace-nowrap ${
                      header.column.getCanSort() ? 'cursor-pointer hover:text-slate-900' : 'cursor-default'
                    }`}
                    onClick={header.column.getToggleSortingHandler()}
                  >
                    <div className="flex items-center gap-1.5">
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                      {header.column.getIsSorted() === 'asc' && ' 🔼'}
                      {header.column.getIsSorted() === 'desc' && ' 🔽'}
                    </div>
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody className="divide-y divide-slate-100">
            {isLoading ? (
              <tr>
                <td colSpan={table.getAllColumns().length} className="py-9 px-4 text-center text-brand font-semibold text-sm">
                  ⏳ Loading...
                </td>
              </tr>
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td colSpan={table.getAllColumns().length} className="py-9 px-4 text-center text-slate-400 font-medium text-sm">
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className={`hover:bg-slate-50/70 transition-colors ${onRowClick ? 'cursor-pointer' : ''}`}
                  onClick={
                    onRowClick
                      ? (e) => {
                          // A click on a button, link or field inside the row is that control's own action.
                          if ((e.target as HTMLElement).closest('a, button, input, select, textarea, label')) return;
                          onRowClick(row.original);
                        }
                      : undefined
                  }
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} className="px-4 py-3 text-slate-800 align-middle">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <DataTablePagination table={table} totalCount={totalCount} />
    </div>
  );
}
