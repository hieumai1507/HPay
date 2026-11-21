interface BarChartProps {
  values: number[];
  labels?: string[];
  height?: number;
}

export function BarChart({ values, labels, height = 120 }: BarChartProps) {
  const max = Math.max(...values, 1);
  return (
    <div>
      <div className="flex items-end gap-1" style={{ height }}>
        {values.map((v, i) => (
          <div
            key={i}
            title={labels?.[i] !== undefined ? `${labels[i]}: ${v}` : String(v)}
            className={`flex-1 rounded-t-xs min-w-[4px] transition-all ${
              v === max ? 'bg-brand' : 'bg-cyan-100 border-t-2 border-cyan-400 hover:bg-cyan-200'
            }`}
            style={{ height: `${Math.max((v / max) * 100, 2)}%` }}
          />
        ))}
      </div>
      {labels && (
        <div className="flex gap-1 mt-1">
          {labels.map((l, i) => (
            <span
              key={i}
              className={`flex-1 text-[9px] text-slate-500 text-center ${
                i % 4 === 0 ? 'visible' : 'invisible'
              }`}
            >
              {l}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
