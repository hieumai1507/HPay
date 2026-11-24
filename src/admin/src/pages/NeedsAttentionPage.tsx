import { Link } from 'react-router-dom';

import { formatAge } from '@/lib/age';
import { formatVnd } from '@/lib/money';
import { useAttentionSummary, useNeedsManualApprovals, useWorkflowInstances } from '@/queries';
import { Badge, ErrorNotice, Table } from '@/ui';

/**
 * G12: parked money cases (approvals needs_manual, workflows compensation_failed).
 * Read-only: this page resolves nothing, it only makes the cases visible.
 */
export default function NeedsAttentionPage() {
  const summary = useAttentionSummary();
  const approvals = useNeedsManualApprovals({ limit: 50 });
  const workflows = useWorkflowInstances({ status: 'compensation_failed', limit: 50 });

  const approvalRows = approvals.data?.rows ?? [];
  const workflowRows = workflows.data?.rows ?? [];
  const loading = approvals.isLoading || workflows.isLoading;
  const empty = !loading && !approvals.error && !workflows.error && approvalRows.length === 0 && workflowRows.length === 0;

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">Needs attention</h1>
        <p className="mt-1 text-sm text-slate-500">
          Cases parked for a person: approvals the system could not apply (needs_manual) and workflows whose
          compensation failed. Nothing here moves money.
        </p>
        {summary.data && <p className="mt-1 text-sm text-slate-700">Open cases: {summary.data.total}</p>}
      </div>

      <ErrorNotice error={approvals.error} what="parked approvals" />
      <ErrorNotice error={workflows.error} what="failed compensations" />
      {loading && <div className="text-sm text-slate-500">Loading…</div>}
      {empty && (
        <div role="status" className="bg-white border border-slate-200 rounded-xl p-6 text-center text-slate-500">
          Nothing needs attention.
        </div>
      )}

      {approvalRows.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-base font-bold text-slate-800">Approvals parked as needs_manual ({approvals.data?.total ?? 0})</h2>
          <Table
            rows={approvalRows}
            columns={[
              { key: 'type', header: 'Type', render: (r) => <Badge tone="danger">{r.subjectType}</Badge> },
              { key: 'ref', header: 'Ref code', render: (r) => <span className="font-mono">{r.refCode || '—'}</span> },
              { key: 'amount', header: 'Amount', render: (r) => (r.amount ? formatVnd(r.amount) : '—') },
              { key: 'age', header: 'Age', render: (r) => formatAge(r.parkedAt) },
              { key: 'reason', header: 'Reason', render: (r) => r.reason || '—' },
              {
                key: 'link',
                header: '',
                render: () => (
                  <Link className="text-brand font-semibold" to="/audit">
                    Audit log
                  </Link>
                ),
              },
            ]}
          />
        </section>
      )}

      {workflowRows.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-base font-bold text-slate-800">Workflows in compensation_failed ({workflows.data?.total ?? 0})</h2>
          <Table
            rows={workflowRows}
            columns={[
              { key: 'def', header: 'Workflow', render: (w) => <Badge tone="danger">{w.definitionId}</Badge> },
              { key: 'ref', header: 'Ref code', render: (w) => <span className="font-mono">{w.txnRef || '—'}</span> },
              { key: 'amount', header: 'Amount', render: (w) => (w.amount ? formatVnd(w.amount) : '—') },
              { key: 'age', header: 'Age', render: (w) => formatAge(w.createdAt) },
              { key: 'reason', header: 'Reason', render: (w) => `Compensation failed at step ${w.currentStep || 'unknown'}` },
              {
                key: 'link',
                header: '',
                render: (w) => (
                  <Link className="text-brand font-semibold" to={`/workflows/${w.id}`}>
                    Detail
                  </Link>
                ),
              },
            ]}
          />
        </section>
      )}
    </div>
  );
}