import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useWorkflowDetail, useDecideWorkflow } from '@/queries';
import { Timeline } from '@/ui';

export default function WorkflowDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: instance, isLoading } = useWorkflowDetail(id);
  const decideMutation = useDecideWorkflow();

  const [reason, setReason] = useState('');
  const [feedback, setFeedback] = useState<string | null>(null);

  if (isLoading || !instance) {
    return <div className="p-10 text-center text-slate-500">Đang tải chi tiết workflow…</div>;
  }

  const handleAction = async (action: 'retry' | 'compensate') => {
    if (!reason.trim()) {
      alert('Vui lòng nhập lý do thao tác (bắt buộc).');
      return;
    }
    try {
      await decideMutation.mutateAsync({ id: instance.id, action, reason: reason.trim() });
      setFeedback(
        action === 'retry'
          ? '✅ Đã phát lệnh Retry thành công! Hệ thống sẽ thử lại bước hiện tại.'
          : '↩ Đã phát lệnh Bồi hoàn (compensate) thành công! Các bút toán trước đã được đảo.',
      );
      setReason('');
    } catch (e: unknown) {
      alert(`Lỗi: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  const steps = instance.steps || [];
  const failedStep = steps.find((s) => s.status === 'fail');

  return (
    <div className="flex flex-col gap-4">
      {/* Topbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-3 flex-wrap">
          <button
            type="button"
            onClick={() => navigate('/workflows')}
            className="px-3 py-1.5 rounded-lg border border-slate-200 bg-white text-xs font-semibold text-slate-700 cursor-pointer hover:bg-slate-50"
          >
            ← Monitor
          </button>
          <h2 className="text-base font-bold text-slate-900">{instance.id} · {instance.definitionId}</h2>
          <span className="bg-rose-50 text-rose-600 text-xs font-bold px-2.5 py-0.5 rounded-full">
            {instance.status.toUpperCase()} {failedStep ? `tại ${failedStep.name}` : ''}
          </span>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            type="button"
            onClick={() => handleAction('retry')}
            disabled={decideMutation.isPending}
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
          >
            ⟳ Retry {failedStep?.name || 'bước'}
          </button>
          <button
            type="button"
            onClick={() => handleAction('compensate')}
            disabled={decideMutation.isPending}
            className="h-9 px-4 rounded-lg bg-white border border-rose-600 text-rose-600 font-semibold text-xs cursor-pointer hover:bg-rose-50"
          >
            Force fail + bồi hoàn
          </button>
        </div>
      </div>

      {feedback && (
        <div className="bg-emerald-50 border-l-4 border-emerald-600 text-emerald-700 p-3.5 rounded-r-lg text-sm font-semibold">
          {feedback}
        </div>
      )}

      {/* Main Content Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Left main area */}
        <div className="lg:col-span-2 flex flex-col gap-4">
          {/* Summary Card */}
          <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs">
            <div className="flex items-center justify-between flex-wrap gap-4">
              <div>
                <div className="text-xs text-slate-500">Giao dịch</div>
                <div className="font-mono font-semibold text-slate-900">{instance.txnRef || '—'}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Số tiền</div>
                <div className="font-mono font-bold text-slate-900 text-base">
                  {instance.amount ? `${instance.amount.toLocaleString('vi-VN')} ₫` : '—'}
                </div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Loại workflow</div>
                <div className="font-semibold text-xs text-slate-800">{instance.definitionId}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Bắt đầu</div>
                <div className="font-medium text-xs text-slate-800">{new Date(instance.createdAt).toLocaleTimeString('vi-VN')}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Context</div>
                <div className="font-mono text-slate-700">{Object.keys(instance.context || {}).length} keys</div>
              </div>
            </div>
          </div>

          {/* Timeline Card */}
          <div className="bg-white rounded-xl p-5 border border-slate-200 shadow-xs">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider mb-3 uppercase">TIMELINE STEP EXECUTIONS</div>
            <Timeline steps={steps} />
          </div>
        </div>

        {/* Right Sidebar */}
        <div className="flex flex-col gap-4">
          {/* Input / Output Code Viewer Card */}
          <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider mb-3 uppercase">INPUT / OUTPUT — {failedStep?.name || 'CONTEXT'}</div>
            <div className="bg-slate-900 rounded-lg p-3.5 overflow-x-auto">
              <pre className="m-0 text-cyan-300 font-mono text-xs leading-relaxed">
                <code>
                  {JSON.stringify(instance.context || { error: failedStep?.error }, null, 2)}
                </code>
              </pre>
            </div>
          </div>

          {/* Action Control Card */}
          <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider mb-1 uppercase">HÀNH ĐỘNG KHẢ DỤNG</div>
            <div className="text-xs text-slate-500 mb-3">
              Theo state machine, instance <b>{instance.status}</b> tại bước có <b>on_failure: compensate</b>:
            </div>

            <div className="flex flex-col gap-2.5">
              <button
                type="button"
                onClick={() => handleAction('retry')}
                disabled={decideMutation.isPending}
                className="h-10.5 w-full rounded-lg bg-brand text-white font-semibold text-sm cursor-pointer hover:bg-brand-600 shadow-xs"
              >
                ⟳ Retry {failedStep?.name || 'bước'}
              </button>
              <button
                type="button"
                onClick={() => handleAction('compensate')}
                disabled={decideMutation.isPending}
                className="h-10.5 w-full rounded-lg bg-rose-50 border border-rose-600 text-rose-600 font-semibold text-sm cursor-pointer hover:bg-rose-100 transition-colors"
              >
                ↩ Bồi hoàn ngay (đảo bút toán)
              </button>
            </div>

            <div className="mt-3.5">
              <label htmlFor="reason-textarea" className="text-xs text-slate-500 block mb-1">Lý do thao tác (bắt buộc):</label>
              <textarea
                id="reason-textarea"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Vd: Đã xử lý ví bị khóa, cho phép retry..."
                className="w-full min-h-[70px] rounded-lg border border-slate-200 p-2.5 text-xs font-sans focus:outline-none focus:ring-2 focus:ring-brand/30"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
