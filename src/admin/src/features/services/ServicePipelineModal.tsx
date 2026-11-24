import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Modal, Badge } from '@/ui';
import type { ServiceItem } from '@/queries';

export interface ActionStepConfig {
  code: string;
  name: string;
  type: 'GL Posting' | 'Internal Ingression' | 'Directional Integration';
  entry_point: string;
  method: string;
  entry_conditions?: string;
  required: boolean;
  has_enquiry?: boolean;
  params?: { name: string; source: string; field: string; default_value?: string }[];
  output?: { name: string; source: string; field: string; need_secured: boolean; map_to_txn: string }[];
  error_mapping?: { code: string; value: string; mapping: string; message: string }[];
}

export interface InputFieldRule {
  order: number;
  name: string;
  rule: 'mapping' | 'query' | 'fixed';
  type: string;
  min_length?: number;
  max_length?: number;
  regex?: string;
  required: boolean;
}

interface Props {
  open: boolean;
  onClose: () => void;
  service: ServiceItem | null;
}

export function ServicePipelineModal({ open, onClose, service }: Props) {
  const [activeTab, setActiveTab] = useState<'overview' | 'definition' | 'validation' | 'pipeline' | 'offer'>('overview');
  const [selectedActionIndex, setSelectedActionIndex] = useState(0);

  // The modal component stays mounted while the list page keeps it, so its local state would carry
  // over from one service to the next. Start every service on the first tab and the first step.
  const serviceId = service?.id;
  useEffect(() => {
    setActiveTab('overview');
    setSelectedActionIndex(0);
  }, [serviceId]);

  if (!service) return null;

  // Only what the API returned: no invented defaults. A service without actions or input fields
  // shows an empty state instead of another service's (or made-up) pipeline.
  const actions: ActionStepConfig[] = Array.isArray(service.actions) ? service.actions : [];
  const inputFields: InputFieldRule[] = Array.isArray(service.inputFields) ? service.inputFields : [];
  const currentAction: ActionStepConfig | undefined = actions[selectedActionIndex] ?? actions[0];

  return (
    <Modal open={open} onClose={onClose} size="lg" title={`Transaction Design Manager · ${service.code}`}>
      <div key={service.id} className="flex flex-col w-full min-w-0">
        {/* Tab Bar */}
        <div className="flex gap-2 border-b border-slate-200 mb-3.5 overflow-x-auto">
          <button
            type="button"
            onClick={() => setActiveTab('overview')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors whitespace-nowrap cursor-pointer ${
              activeTab === 'overview' ? 'border-brand text-brand' : 'border-transparent text-slate-600 hover:text-slate-900'
            }`}
          >
            📋 Overview
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('definition')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors whitespace-nowrap cursor-pointer ${
              activeTab === 'definition' ? 'border-brand text-brand' : 'border-transparent text-slate-600 hover:text-slate-900'
            }`}
          >
            🏦 Trans Definition (GL)
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('validation')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors whitespace-nowrap cursor-pointer ${
              activeTab === 'validation' ? 'border-brand text-brand' : 'border-transparent text-slate-600 hover:text-slate-900'
            }`}
          >
            🛡️ Trans Validations ({inputFields.length})
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('pipeline')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors whitespace-nowrap cursor-pointer ${
              activeTab === 'pipeline' ? 'border-brand text-brand' : 'border-transparent text-slate-600 hover:text-slate-900'
            }`}
          >
            ⚡ Action Pipeline ({actions.length} steps)
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('offer')}
            className={`px-3 py-2 text-xs font-semibold border-b-2 transition-colors whitespace-nowrap cursor-pointer ${
              activeTab === 'offer' ? 'border-brand text-brand' : 'border-transparent text-slate-600 hover:text-slate-900'
            }`}
          >
            🔑 Offer Auth Matrix
          </button>
        </div>

        {/* Tab 0: Overview */}
        {activeTab === 'overview' && (
          <div className="flex flex-col gap-3 min-h-[300px]">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Service code</label>
                <div className="h-9 flex items-center px-3 bg-slate-100 rounded-lg text-xs font-bold font-mono text-slate-900">{service.code}</div>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Service name</label>
                <div className="h-9 flex items-center px-3 bg-slate-100 rounded-lg text-xs font-bold text-slate-900">{service.name}</div>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Full name</label>
                <input type="text" defaultValue={service.fullName || service.name} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white" />
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Transaction type</label>
                <select defaultValue={service.transactionType || 'Normal Transaction'} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white">
                  <option value="Normal Transaction">Normal Transaction</option>
                  <option value="Refund">Refund</option>
                  <option value="Inquiry">Inquiry</option>
                  <option value="Reversal">Reversal</option>
                </select>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Action priority</label>
                <select defaultValue={service.actionPriority} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white">
                  <option value="before_gl">Before: Call 3rd party first then Post to GL</option>
                  <option value="after_gl">After: Post to GL first then call 3rd party</option>
                </select>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">GL posting confirmation</label>
                <div className="flex items-center gap-2.5 h-9">
                  <input type="checkbox" defaultChecked={service.confirmation !== false} id="gl-check" className="rounded border-slate-300 text-brand" />
                  <label htmlFor="gl-check" className="text-xs font-semibold text-slate-800">Create double-entry ledger posting</label>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 1: Trans Definition (GL Posting) */}
        {activeTab === 'definition' && (
          <div className="flex flex-col items-center justify-center gap-1 min-h-[300px] text-center text-slate-500">
            <div className="text-sm font-bold text-slate-700">Ledger rules are not shown here yet</div>
            <div className="text-xs max-w-md">
              The debit and credit rules are defined in code, not stored per service, so there is nothing real to display.
              Every posting is double-entry and is checked against the ledger.
            </div>
          </div>
        )}
        {/* Tab 2: Validations */}
        {activeTab === 'validation' && (
          <div className="flex flex-col gap-3 min-h-[300px]">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">INPUT FIELDS &amp; VALIDATION RULES</div>
            <table className="w-full border-collapse text-xs bg-white rounded-lg overflow-hidden border border-slate-200">
              <thead>
                <tr className="bg-slate-50 border-b border-slate-200 text-slate-600">
                  <th className="p-2.5 text-left font-bold">Order</th>
                  <th className="p-2.5 text-left font-bold">Field Name</th>
                  <th className="p-2.5 text-left font-bold">Rule</th>
                  <th className="p-2.5 text-left font-bold">Type</th>
                  <th className="p-2.5 text-left font-bold">Min / Max Length</th>
                  <th className="p-2.5 text-left font-bold">Regex Pattern</th>
                  <th className="p-2.5 text-left font-bold">Required</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {inputFields.length === 0 && (
                  <tr>
                    <td colSpan={7} className="p-6 text-center text-slate-500">No input fields configured for this service.</td>
                  </tr>
                )}
                {inputFields.map((f) => (
                  <tr key={f.name}>
                    <td className="p-2.5 font-bold text-slate-700">#{f.order}</td>
                    <td className="p-2.5 font-mono font-semibold text-slate-900">{f.name}</td>
                    <td className="p-2.5"><Badge tone={f.rule === 'mapping' ? 'info' : 'warning'}>{f.rule}</Badge></td>
                    <td className="p-2.5 text-slate-700">{f.type}</td>
                    <td className="p-2.5 text-slate-700">{f.min_length || 0} - {f.max_length || '∞'}</td>
                    <td className="p-2.5 font-mono text-slate-600">{f.regex || '—'}</td>
                    <td className="p-2.5">{f.required ? <span className="text-emerald-600 font-bold">✓ Yes</span> : 'No'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Tab 3: Action Pipeline */}
        {activeTab === 'pipeline' && actions.length === 0 && (
          <div className="flex flex-col items-center justify-center gap-1 min-h-[300px] text-center text-slate-500">
            <div className="text-sm font-bold text-slate-700">No actions configured</div>
            <div className="text-xs">This service has no third-party or ledger actions in its pipeline.</div>
          </div>
        )}
        {activeTab === 'pipeline' && currentAction && (
          <div className="flex flex-col md:flex-row gap-4 min-h-[300px]">
            <div className="w-full md:w-64 flex flex-col gap-2.5 shrink-0">
              <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">ACTION STEPS</div>
              {actions.map((act, i) => (
                <div
                  key={i}
                  onClick={() => setSelectedActionIndex(i)}
                  className={`bg-white rounded-lg p-2.5 cursor-pointer shadow-xs border transition-all ${
                    i === selectedActionIndex ? 'border-brand border-2' : 'border-slate-200'
                  }`}
                >
                  <div className="flex justify-between items-center">
                    <span className="text-[11px] font-bold text-brand">Step {i + 1}</span>
                    <Badge tone={act.type === 'GL Posting' ? 'info' : act.type === 'Directional Integration' ? 'warning' : 'success'}>
                      {act.type}
                    </Badge>
                  </div>
                  <div className="text-xs font-bold text-slate-900 mt-1">{act.name}</div>
                </div>
              ))}
            </div>

            <div className="flex-1 flex flex-col gap-3 bg-slate-50 p-3.5 rounded-xl border border-slate-200">
              <h4 className="m-0 text-sm font-bold text-slate-900">Step {selectedActionIndex + 1}: {currentAction.name}</h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="text-[11px] font-bold text-slate-500 mb-1 block">Entry Point (Endpoint)</label>
                  <input type="text" key={`${service.id}-${selectedActionIndex}-ep`} defaultValue={currentAction.entry_point} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white" />
                </div>
                <div>
                  <label className="text-[11px] font-bold text-slate-500 mb-1 block">Method</label>
                  <input type="text" key={`${service.id}-${selectedActionIndex}-m`} defaultValue={currentAction.method} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white" />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 4: Offer Auth Matrix */}
        {activeTab === 'offer' && (
          <div className="flex flex-col items-center justify-center gap-2 min-h-[300px] text-center text-slate-500">
            <div className="text-sm font-bold text-slate-700">The authentication matrix is set per plan</div>
            <div className="text-xs max-w-md">
              Which authentication a payment needs (PIN, OTP code or two-step) depends on the customer's plan and the
              amount, so it is shown on each plan, not on the service.
            </div>
            <Link to="/offers" className="text-xs font-semibold text-brand hover:underline">Open Offer Factory →</Link>
          </div>
        )}
        {/* Footer */}
        <div className="flex justify-end gap-2.5 mt-4 pt-3 border-t border-slate-200">
          <button type="button" onClick={onClose} className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50">Close</button>
          <button
            type="button"
            disabled
            title="Not available yet: the backend has no endpoint to save a service's transaction design (only PUT /v1/admin/services/{id} with a status)."
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs shadow-xs opacity-60 cursor-not-allowed"
          >
            Save transaction design
          </button>
        </div>
      </div>
    </Modal>
  );
}
