import { Pressable, ScrollView, Text } from 'react-native';

export interface FilterChipsProps<T extends string> {
  options: { value: T; label: string }[];
  selected: T;
  onSelect: (value: T) => void;
}

export function FilterChips<T extends string>({ options, selected, onSelect }: FilterChipsProps<T>) {
  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      className="flex-grow-0"
      contentContainerStyle={{ flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 16, paddingVertical: 8 }}
    >
      {options.map((o) => {
        const active = o.value === selected;
        return (
          <Pressable
            key={o.value}
            onPress={() => onSelect(o.value)}
            accessibilityRole="button"
            accessibilityState={{ selected: active }}
            className={`py-1.5 px-4 rounded-full min-h-[32px] justify-center ${active ? 'bg-brand' : 'bg-slate-100 dark:bg-slate-800'}`}
          >
            <Text className={`text-xs font-semibold ${active ? 'text-white' : 'text-slate-700 dark:text-slate-200'}`}>{o.label}</Text>
          </Pressable>
        );
      })}
    </ScrollView>
  );
}
