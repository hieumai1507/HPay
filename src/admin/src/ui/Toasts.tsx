import { useToastStore } from '@/stores/toast';

const toneClasses = {
  success: { bg: 'bg-emerald-50 border-emerald-600 text-emerald-700', icon: '✓' },
  error: { bg: 'bg-rose-50 border-rose-600 text-rose-700', icon: '✕' },
  info: { bg: 'bg-cyan-50 border-cyan-600 text-cyan-800', icon: 'ℹ' },
} as const;

export function Toasts() {
  const toasts = useToastStore((s) => s.toasts);
  const dismiss = useToastStore((s) => s.dismiss);

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-5 right-5 flex flex-col gap-2 z-[100] max-w-sm w-full">
      {toasts.map((t) => {
        const c = toneClasses[t.tone];
        return (
          <div
            key={t.id}
            onClick={() => dismiss(t.id)}
            role="status"
            className={`border-l-4 rounded-lg p-3 text-sm font-medium shadow-lg cursor-pointer transition-all hover:opacity-90 text-slate-900 ${c.bg}`}
          >
            <span className="font-bold mr-2">{c.icon}</span>
            {t.message}
          </div>
        );
      })}
    </div>
  );
}
