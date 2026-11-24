import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useWorkflowDetail, useDecideWorkflow } from '@/queries';
import { Timeline } from '@/ui';
import { NO_PERMISSION_REASON } from '@/lib/access';
import { useAdminStore } from '@/stores/admin';

export default function WorkflowDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: instance, isLoading } = useWorkflowDetail(id);
  const decideMutation = useDecideWorkflow();
  // UX only: the backend does not enforce admin roles yet (audit BE-H6).
  const canAct = useAdminStore((s) => s.can('workflow.action'));

  const [reason, setReason] = useState('');
  const [feedback, setFeedback] = useState<string | null>(null);

  if (isLoading || !instance) {
    return <div className="p-10 text-center text-slate-500">Loading...</div>;
  }

  const handleAction = async (action: 'retry' | 'compensate') => {
    if (decideMutation.isPending || !canAct) return;
    if (!reason.trim()) {
      alert('Please enter a reason for this action (required).');
      return;
    }
    try {
      await decideMutation.mutateAsync({ id: instance.id, action, reason: reason.trim() });
      setFeedback(
        action === 'retry'
          ? '✅ Retry command sent. The system will retry the current step.'
          : '↩ Compensate command sent. Earlier ledger entries have been reversed.',
      );
      setReason('');
    } catch (e: unknown) {
      alert(`Error: ${e instanceof Error ? e.message : String(e)}`);
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
          <h2 className="text-base font-bold text-slate-900">
            {instance.txnRef !== '—' ? instance.txnRef : 'Workflow'} · {instance.definitionId}
          </h2>
          <span className="bg-rose-50 text-rose-600 text-xs font-bold px-2.5 py-0.5 rounded-full">
            {instance.status.replace(/_/g, ' ').toUpperCase()} {failedStep ? `at ${failedStep.name}` : ''}
          </span>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            type="button"
            onClick={() => handleAction('retry')}
            disabled={decideMutation.isPending || !canAct}
            title={canAct ? undefined : NO_PERMISSION_REASON}
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
          >
            ⟳ Retry {failedStep?.name || 'step'}
          </button>
          <button
            type="button"
            onClick={() => handleAction('compensate')}
            disabled={decideMutation.isPending || !canAct}
            title={canAct ? undefined : NO_PERMISSION_REASON}
            className="h-9 px-4 rounded-lg bg-white border border-rose-600 text-rose-600 font-semibold text-xs cursor-pointer hover:bg-rose-50"
          >
            Force fail + compensate
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
                <div className="text-xs text-slate-500">Transaction</div>
                <div className="font-mono font-semibold text-slate-900">{instance.txnRef || '—'}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Amount</div>
                <div className="font-mono font-bold text-slate-900 text-base">
                  {instance.amount ? `${instance.amount.toLocaleString('en-US')} ₫` : '—'}
                </div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Process</div>
                <div className="font-semibold text-xs text-slate-800">{instance.definitionId}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Started</div>
                <div className="font-medium text-xs text-slate-800">{new Date(instance.createdAt).toLocaleTimeString('en-GB')}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500">Context</div>
                <div className="font-mono text-slate-700">{Object.keys(instance.context ?? {}).length} {Object.keys(instance.context ?? {}).length === 1 ? 'item' : 'items'}</div>
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
            <div className="text-[11px] font-bold text-slate-500 tracking-wider mb-3 uppercase">DATA — {failedStep?.name || 'WORKFLOW'}</div>
            <div className="bg-slate-900 rounded-lg p-3.5 overflow-x-auto">
              <pre className="m-0 text-cyan-300 font-mono text-xs leading-relaxed">
                <code>
                  {JSON.stringify(failedStep?.output ?? instance.context ?? {}, null, 2)}
                </code>
              </pre>
            </div>
          </div>

          {/* Action Control Card */}
          <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider mb-1 uppercase">AVAILABLE ACTIONS</div>
            <div className="text-xs text-slate-500 mb-3">
              This payment is <b>{instance.status.replace(/_/g, ' ')}</b>. You can try the failed step again, or reverse the money that already moved:
            </div>

            <div className="flex flex-col gap-2.5">
              <button
                type="button"
                onClick={() => handleAction('retry')}
                disabled={decideMutation.isPending || !canAct}
            title={canAct ? undefined : NO_PERMISSION_REASON}
                className="h-10.5 w-full rounded-lg bg-brand text-white font-semibold text-sm cursor-pointer hover:bg-brand-600 shadow-xs"
              >
                ⟳ Retry {failedStep?.name || 'step'}
              </button>
              <button
                type="button"
                onClick={() => handleAction('compensate')}
                disabled={decideMutation.isPending || !canAct}
            title={canAct ? undefined : NO_PERMISSION_REASON}
                className="h-10.5 w-full rounded-lg bg-rose-50 border border-rose-600 text-rose-600 font-semibold text-sm cursor-pointer hover:bg-rose-100 transition-colors"
              >
                ↩ Compensate now (reverse entries)
              </button>
            </div>

            <div className="mt-3.5">
              <label htmlFor="reason-textarea" className="text-xs text-slate-500 block mb-1">Reason for this action (required):</label>
              <textarea
                id="reason-textarea"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="e.g. Locked wallet resolved, retry allowed..."
                className="w-full min-h-[70px] rounded-lg border border-slate-200 p-2.5 text-xs font-sans focus:outline-none focus:ring-2 focus:ring-brand/30"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
