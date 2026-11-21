# 📊 07 — Plan: Standardizing TanStack Table v8 & TanStack Query Pagination

Based on a direct reference to the `hecci-develop` source code, this is a detailed plan for building the optimal paginated table module (Server-Side Pagination, Sorting, Filtering) shared by **hpay-admin** and **hpay-mobile**.

## 1. Overall Architecture (Modular Table Architecture)

```
src/admin/src/
├── components/table/
│   ├── DataTable.tsx             # Component rendering the TanStack Table v8 table
│   ├── DataTablePagination.tsx   # Pagination bar component (Page 1,2.. | 10,20,50 rows)
│   ├── DataTableToolbar.tsx      # Toolbar: Search + Status filter + Date Range
│   └── DataTableRowActions.tsx   # Per-row action menu (View, Edit, Approve)
├── hooks/
│   ├── useTableQueryState.ts     # Hook managing pagination State (pageIndex, pageSize, search, sorting)
│   └── useServerTableQuery.ts    # Hook connecting TanStack Query v5 to fetch a paginated API
```

## 2. State Sync Rules (`useTableQueryState`)

- **Zero-Based vs One-Based**: TanStack Table uses `pageIndex: 0` (zero-based), while the Backend API uses `page: 1` (one-based). The hook automatically converts `page = pageIndex + 1`.
- **Auto-Reset Page**: When the user types a search keyword (Global Filter) or selects a new Filter, the `pageIndex` state automatically resets to `0` (the first page).
- **Manual Server-Side Flags**: Declare `manualPagination: true`, `manualSorting: true`, `manualFiltering: true` so that TanStack Table hands all filtering & pagination duties to the Backend SQL/Index.

## 3. Detailed Execution Tasks

**Task 1: Create the standard TanStack Table Module in `hpay-admin`**

Install the `@tanstack/react-table` package and create the component set `DataTable.tsx`, `DataTablePagination.tsx`, `DataTableToolbar.tsx` at `src/components/table/`.

**Task 2: Create State & Query Management Hooks in `hpay-admin`**

Write `useTableQueryState.ts` and `useServerTableQuery.ts` to automatically connect TanStack Query with the paginated Backend API (carrying the parameters `total`, `page`, `limit`, `total_pages`).

**Task 3: Update the Admin Ops Pages to Use the New Table**

Convert the **Audit Logs**, **KYC Queue**, **Approval Inbox**, **Service Catalog**, **Fraud Flags** pages to use the new `DataTable`.

**Task 4: Optimized Pagination for Mobile (`hpay-mobile`)**

Create a `usePaginatedList` hook combining TanStack Query `useInfiniteQuery` / `useQuery` with `FlatList` / `FlashList`, supporting Pull-to-Refresh & Load More on scroll.
