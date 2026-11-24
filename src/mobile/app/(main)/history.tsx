import { router } from 'expo-router';
import { useMemo, useState } from 'react';
import { Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { FlashList } from '@shopify/flash-list';

import { useTransactions, type TxnView } from '@/src/queries';
import { describeError } from '@/src/lib/errors';
import {
  EmptyState,
  ErrorState,
  FilterChips,
  LoadingFooter,
  Skeleton,
  TxnListItem,
} from '@/src/ui';

type Filter = 'all' | 'in' | 'out' | 'processing';

const FILTERS: { value: Filter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'in', label: 'Money in' },
  { value: 'out', label: 'Money out' },
  { value: 'processing', label: 'Processing' },
];

const PROCESSING_STATUSES = ['created', 'processing', 'pending_approval'];

function matchesFilter(t: TxnView, f: Filter): boolean {
  switch (f) {
    case 'in':
      return t.direction === 'in';
    case 'out':
      return t.direction === 'out';
    case 'processing':
      return PROCESSING_STATUSES.includes(t.status);
    default:
      return true;
  }
}

function dayLabel(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const startOfDay = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diffDays = Math.round((startOfDay(now) - startOfDay(d)) / 86_400_000);
  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return 'Yesterday';
  return d.toLocaleDateString('en-GB');
}

interface HeaderItem {
  type: 'header';
  title: string;
}

interface RowItem {
  type: 'row';
  txn: TxnView;
}

type ListItem = HeaderItem | RowItem;

const ActiveFlashList = FlashList as any;

export default function HistoryScreen() {
  const [filter, setFilter] = useState<Filter>('all');
  const q = useTransactions();

  const listData = useMemo<ListItem[]>(() => {
    const all = q.data?.pages.flatMap((p) => p.transactions) ?? [];
    const filtered = all.filter((t) => matchesFilter(t, filter));
    const byDay = new Map<string, TxnView[]>();
    for (const t of filtered) {
      const key = dayLabel(t.created_at);
      const bucket = byDay.get(key);
      if (bucket) bucket.push(t);
      else byDay.set(key, [t]);
    }

    const items: ListItem[] = [];
    byDay.forEach((transactions, day) => {
      items.push({ type: 'header', title: day });
      for (const t of transactions) {
        items.push({ type: 'row', txn: t });
      }
    });
    return items;
  }, [q.data, filter]);

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <Text className="text-2xl font-bold text-slate-900 dark:text-white px-4 pt-2 mb-2">History</Text>
      <FilterChips options={FILTERS} selected={filter} onSelect={setFilter} />

      {q.isLoading ? (
        <Skeleton />
      ) : q.isError ? (
        <ErrorState message={describeError(q.error)} onRetry={() => q.refetch()} />
      ) : (
        <View className="flex-1 w-full mt-2">
          <ActiveFlashList
            data={listData}
            keyExtractor={(item: ListItem) => {
              if (item.type === 'header') return `header-${item.title}`;
              return item.txn.id;
            }}
            renderItem={({ item }: { item: ListItem }) => {
              if (item.type === 'header') {
                return <Text className="text-xs font-semibold text-slate-500 bg-slate-100 dark:bg-slate-800 px-4 py-2 uppercase tracking-wider">{item.title}</Text>;
              }
              return (
                <TxnListItem
                  txn={item.txn}
                  onPress={(refCode) =>
                    router.push({ pathname: '/(main)/txn-detail', params: { refCode } })
                  }
                />
              );
            }}
            getItemType={(item: ListItem) => item.type}
            estimatedItemSize={72}
            ListEmptyComponent={
              <EmptyState
                title="Nothing here yet"
                subtitle={filter === 'all' ? 'Your Transactions will show up here.' : 'No Transactions match the filter.'}
              />
            }
            ListFooterComponent={q.isFetchingNextPage ? <LoadingFooter /> : null}
            onEndReachedThreshold={0.4}
            onEndReached={() => {
              if (q.hasNextPage && !q.isFetchingNextPage) q.fetchNextPage();
            }}
            refreshing={q.isRefetching && !q.isFetchingNextPage}
            onRefresh={() => q.refetch()}
            contentContainerStyle={{ paddingBottom: 24 }}
          />
        </View>
      )}
    </SafeAreaView>
  );
}
