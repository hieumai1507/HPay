export interface TimelineStepItem {
  id?: string;
  name: string;
  status: 'done' | 'fail' | 'active' | 'wait' | 'running';
  durationMs?: number;
  attempt?: number;
  maxAttempts?: number;
  output?: Record<string, unknown> | string | null;
  error?: string | null;
  /** Unedited error text, shown as a tooltip. */
  rawError?: string | null;
  timestamp?: string;
}

interface TimelineProps {
  steps: TimelineStepItem[];
}

export function Timeline({ steps }: TimelineProps) {
  return (
    <div className="flex flex-col">
      {steps.map((step, index) => {
        const isLast = index === steps.length - 1;
        return (
          <div key={step.id || `${step.name}-${index}`} className="flex gap-3.5">
            {/* Rail line and dot */}
            <div className="flex flex-col items-center w-6">
              <div className={`w-5.5 h-5.5 rounded-full flex items-center justify-center text-xs font-bold shrink-0 border z-10 ${getDotClasses(step.status)}`}>
                {renderDotIcon(step.status)}
              </div>
              {!isLast && <div className={`w-0.5 flex-1 my-0.5 ${step.status === 'done' ? 'bg-emerald-500' : 'bg-slate-200'}`} />}
            </div>

            {/* Step Body */}
            <div className={`flex-1 ${isLast ? 'pb-0' : 'pb-5'}`}>
              <div className="flex items-center gap-2.5 flex-wrap">
                <b className={`text-sm ${step.status === 'fail' ? 'text-rose-600' : 'text-slate-900'}`}>
                  {step.name}
                </b>
                {step.durationMs !== undefined && (
                  <span className="text-xs text-slate-500">
                    {step.durationMs < 1000 ? `${step.durationMs}ms` : `${(step.durationMs / 1000).toFixed(1)}s`}
                  </span>
                )}
                {step.attempt && step.attempt > 1 && (
                  <span className={`text-[11px] font-semibold px-2 py-0.5 rounded-full ${step.status === 'fail' ? 'bg-rose-50 text-rose-600' : 'bg-amber-50 text-amber-600'}`}>
                    attempt {step.attempt}{step.maxAttempts ? `/${step.maxAttempts}` : ''}
                  </span>
                )}
                {step.timestamp && <span className="text-xs text-slate-500">{step.timestamp}</span>}
              </div>

              {step.output && (typeof step.output === 'string' || Object.keys(step.output).length > 0) && (
                <div className="font-mono text-xs text-slate-500 mt-1 break-all">
                  Result: {typeof step.output === 'string' ? step.output : JSON.stringify(step.output)}
                </div>
              )}

              {step.error && (
                <div className="text-xs text-rose-600 font-semibold mt-1 break-words" title={step.rawError ?? undefined}>
                  {step.error}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function renderDotIcon(status: TimelineStepItem['status']) {
  switch (status) {
    case 'done':
      return '✓';
    case 'fail':
      return '✕';
    case 'active':
    case 'running':
      return '●';
    default:
      return '';
  }
}

function getDotClasses(status: TimelineStepItem['status']): string {
  switch (status) {
    case 'done':
      return 'bg-emerald-600 text-white border-emerald-600';
    case 'fail':
      return 'bg-rose-600 text-white border-rose-600';
    case 'active':
    case 'running':
      return 'bg-brand text-white border-brand animate-pulse';
    case 'wait':
    default:
      return 'bg-slate-100 text-slate-400 border-slate-300';
  }
}
