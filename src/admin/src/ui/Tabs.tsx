interface Tab<K extends string> {
  key: K;
  label: string;
  count?: number;
}

interface TabsProps<K extends string> {
  tabs: Tab<K>[];
  active: K;
  onChange: (key: K) => void;
}

export function Tabs<K extends string>({ tabs, active, onChange }: TabsProps<K>) {
  return (
    <div className="flex gap-1 border-b-2 border-slate-200">
      {tabs.map((t) => {
        const isActive = t.key === active;
        return (
          <button
            key={t.key}
            onClick={() => onChange(t.key)}
            className={`-mb-[2px] px-4 py-2.5 text-sm font-semibold border-b-2 transition-colors cursor-pointer ${
              isActive
                ? 'border-brand text-brand'
                : 'border-transparent text-slate-500 hover:text-slate-700'
            }`}
          >
            {t.label}
            {t.count !== undefined && (
              <span
                className={`ml-2 px-2 py-0.5 rounded-full text-xs font-bold ${
                  isActive ? 'bg-cyan-50 text-cyan-800' : 'bg-slate-100 text-slate-500'
                }`}
              >
                {t.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
