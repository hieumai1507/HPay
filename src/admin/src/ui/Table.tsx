import { useState } from 'react';
import type { ReactNode, CSSProperties } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';

export interface Column<T> {
  key: string;
  header: ReactNode;
  render: (row: T) => ReactNode;
  sortValue?: (row: T) => string | number;
  width?: number | string;
  align?: 'left' | 'right' | 'center';
}

interface TableProps<T> {
  columns: Column<T>[];
  rows?: T[];
  data?: T[];
  rowKey?: (row: T) => string;
  onRowClick?: (row: T) => void;
  emptyText?: string;
  getRowStyle?: (row: T) => CSSProperties | undefined;
}

export function Table<T>({
  columns,
  rows,
  data,
  rowKey,
  onRowClick,
  emptyText = 'Nothing here yet',
  getRowStyle,
}: TableProps<T>) {
  const [sort, setSort] = useState<{ key: string; dir: 1 | -1 } | null>(null);

  const list = rows ?? data ?? [];
  const getKey = rowKey ?? ((row: T) => String((row as Record<string, unknown>).id ?? JSON.stringify(row)));

  const sorted = (() => {
    if (!sort) return list;
    const col = columns.find((c) => c.key === sort.key);
    if (!col?.sortValue) return list;
    const sv = col.sortValue;
    return [...list].sort((a, b) => {
      const va = sv(a);
      const vb = sv(b);
      return (va < vb ? -1 : va > vb ? 1 : 0) * sort.dir;
    });
  })();

  const toggleSort = (key: string) =>
    setSort((s) => (s?.key === key ? (s.dir === 1 ? { key, dir: -1 } : null) : { key, dir: 1 }));

  return (
    <div className="bg-white border border-slate-200 rounded-xl overflow-x-auto shadow-xs">
      <table className="w-full border-collapse text-sm text-left">
        <thead>
          <tr className="border-b border-slate-200 bg-slate-50">
            {columns.map((col) => (
              <th
                key={col.key}
                onClick={col.sortValue ? () => toggleSort(col.key) : undefined}
                className={`px-3.5 py-2.5 text-xs font-bold uppercase tracking-wider text-slate-500 whitespace-nowrap select-none ${
                  col.sortValue ? 'cursor-pointer hover:text-slate-700' : ''
                } ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left'}`}
                style={{ width: col.width }}
              >
                <span className={`inline-flex items-center gap-1 ${col.align === 'right' ? 'justify-end' : col.align === 'center' ? 'justify-center' : 'justify-start'}`}>
                  {col.header}
                  {sort?.key === col.key &&
                    (sort.dir === 1 ? <ChevronUp size={13} /> : <ChevronDown size={13} />)}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {sorted.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="px-4 py-8 text-center text-slate-400">
                {emptyText}
              </td>
            </tr>
          ) : (
            sorted.map((row) => {
              const customStyle = getRowStyle ? getRowStyle(row) : undefined;
              return (
                <tr
                  key={getKey(row)}
                  onClick={onRowClick ? () => onRowClick(row) : undefined}
                  className={`transition-colors hover:bg-slate-50/80 ${onRowClick ? 'cursor-pointer' : ''}`}
                  style={customStyle}
                >
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      className={`px-3.5 py-3 align-top ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left'}`}
                    >
                      {col.render(row)}
                    </td>
                  ))}
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
