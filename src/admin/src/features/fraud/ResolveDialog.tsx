import React, { useState } from 'react';
import { Modal } from '@/ui';
import type { FraudFlagItem } from '@/queries';

interface ResolveDialogProps {
  flag: FraudFlagItem | null;
  onClose: () => void;
  onConfirm: (payload: { action: 'clear' | 'confirm_fraud'; option: string; reason: string }) => Promise<void>;
  isSubmitting?: boolean;
}

export function ResolveDialog({ flag, onClose, onConfirm, isSubmitting }: ResolveDialogProps) {
  const [selectedOption, setSelectedOption] = useState<'lock_cancel' | 'lock_reversal' | 'mark_only'>('lock_cancel');
  const [reason, setReason] = useState('');

  if (!flag) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (reason.trim().length < 10) {
      alert('The reason must be at least 10 characters.');
      return;
    }
    const action = selectedOption === 'mark_only' ? 'clear' : 'confirm_fraud';
    await onConfirm({
      action,
      option: selectedOption,
      reason: reason.trim(),
    });
  };

  return (
    <Modal open={Boolean(flag)} onClose={onClose} title={`Confirm resolution — flag ${flag.flagNo}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="text-xs text-slate-500 leading-relaxed">
          Choose the consequence to apply to Transaction <b className="font-mono text-slate-900">{flag.txnRef}</b> for user <b>{flag.userName ?? '—'}</b>. This action is recorded in the Audit log and cannot be undone from the UI.
        </div>

        {/* Radio Option 1 */}
        <div
          onClick={() => setSelectedOption('lock_cancel')}
          className={`border-1.5 rounded-xl p-3.5 flex gap-3 cursor-pointer transition-all ${
            selectedOption === 'lock_cancel' ? 'border-brand bg-cyan-50/50' : 'border-slate-200 bg-white hover:bg-slate-50'
          }`}
        >
          <div className={`w-5 h-5 rounded-full border-2 flex items-center justify-center shrink-0 mt-0.5 ${selectedOption === 'lock_cancel' ? 'border-brand' : 'border-slate-300'}`}>
            {selectedOption === 'lock_cancel' && <div className="w-2.5 h-2.5 rounded-full bg-brand" />}
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900">Lock Account + cancel Pending Transactions</div>
            <div className="text-xs text-slate-500 mt-0.5 leading-normal">The user cannot log in; running Workflows will be compensated (saga). Completed Transactions stay unchanged.</div>
          </div>
        </div>

        {/* Radio Option 2 */}
        <div
          onClick={() => setSelectedOption('lock_reversal')}
          className={`border-1.5 rounded-xl p-3.5 flex gap-3 cursor-pointer transition-all ${
            selectedOption === 'lock_reversal' ? 'border-brand bg-cyan-50/50' : 'border-slate-200 bg-white hover:bg-slate-50'
          }`}
        >
          <div className={`w-5 h-5 rounded-full border-2 flex items-center justify-center shrink-0 mt-0.5 ${selectedOption === 'lock_reversal' ? 'border-brand' : 'border-slate-300'}`}>
            {selectedOption === 'lock_reversal' && <div className="w-2.5 h-2.5 rounded-full bg-brand" />}
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900">Lock Account + reverse Completed Transaction</div>
            <div className="text-xs text-slate-500 mt-0.5 leading-normal">Creates a reversal for {flag.txnRef} (reversing entry, the ledger is not deleted). Requires the risk_officer role.</div>
          </div>
        </div>

        {/* Radio Option 3 */}
        <div
          onClick={() => setSelectedOption('mark_only')}
          className={`border-1.5 rounded-xl p-3.5 flex gap-3 cursor-pointer transition-all ${
            selectedOption === 'mark_only' ? 'border-brand bg-cyan-50/50' : 'border-slate-200 bg-white hover:bg-slate-50'
          }`}
        >
          <div className={`w-5 h-5 rounded-full border-2 flex items-center justify-center shrink-0 mt-0.5 ${selectedOption === 'mark_only' ? 'border-brand' : 'border-slate-300'}`}>
            {selectedOption === 'mark_only' && <div className="w-2.5 h-2.5 rounded-full bg-brand" />}
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900">Mark as safe / Record only, no action</div>
            <div className="text-xs text-slate-500 mt-0.5 leading-normal">Keeps the record for further investigation, with no effect on the user or Transactions.</div>
          </div>
        </div>

        {/* Reason Input */}
        <div>
          <label htmlFor="fraud-reason" className="text-xs font-bold text-slate-500 block mb-1.5">Reason (required, at least 10 characters)</label>
          <textarea
            id="fraud-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="E.g. Clear smurfing pattern, 6 Transactions in 4 minutes to 6 new Recipients, user did not answer calls..."
            className="w-full min-h-[72px] rounded-lg border border-slate-200 p-2.5 text-xs font-sans focus:outline-none focus:ring-2 focus:ring-brand/30"
            required
          />
        </div>

        {/* Footer Buttons */}
        <div className="flex justify-end gap-2.5 pt-2">
          <button type="button" onClick={onClose} className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer hover:bg-slate-50">
            Cancel
          </button>
          <button
            type="submit"
            disabled={isSubmitting || reason.trim().length < 10}
            className={`h-9 px-4 rounded-lg border-none text-white text-xs font-semibold cursor-pointer transition-colors shadow-xs ${
              selectedOption === 'mark_only' ? 'bg-emerald-600 hover:bg-emerald-700' : 'bg-rose-600 hover:bg-rose-700'
            } ${isSubmitting || reason.trim().length < 10 ? 'opacity-60 cursor-not-allowed' : ''}`}
          >
            {isSubmitting ? 'Processing...' : selectedOption === 'mark_only' ? 'Mark as safe' : 'Confirm Fraud resolution'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
