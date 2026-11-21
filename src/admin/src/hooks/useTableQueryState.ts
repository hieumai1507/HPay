import { useState, useMemo, useEffect } from 'react';
import type { ColumnFiltersState, PaginationState, SortingState } from '@tanstack/react-table';

export interface UseTableQueryStateOptions {
  initialPageSize?: number;
  initialSortField?: string;
  initialSortDesc?: boolean;
}

export function useTableQueryState(options?: UseTableQueryStateOptions) {
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: options?.initialPageSize ?? 10,
  });

  const [sorting, setSorting] = useState<SortingState>([
    {
      id: options?.initialSortField ?? 'created_at',
      desc: options?.initialSortDesc ?? true,
    },
  ]);

  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [globalFilter, setGlobalFilter] = useState<string>('');

  // Auto-reset pageIndex to 0 when global search or column filters change
  useEffect(() => {
    setPagination((prev) => (prev.pageIndex === 0 ? prev : { ...prev, pageIndex: 0 }));
  }, [globalFilter, columnFilters]);

  // Query parameters formatted for Backend 1-based API calls
  const queryParams = useMemo(() => {
    return {
      page: pagination.pageIndex + 1,
      limit: pagination.pageSize,
      search: globalFilter || undefined,
      sort: sorting[0]?.id,
      order: sorting[0]?.desc ? 'desc' : 'asc',
    };
  }, [pagination.pageIndex, pagination.pageSize, globalFilter, sorting]);

  return {
    pagination,
    setPagination,
    sorting,
    setSorting,
    columnFilters,
    setColumnFilters,
    globalFilter,
    setGlobalFilter,
    queryParams,
  };
}
