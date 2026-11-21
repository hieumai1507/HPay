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
      alert('Lý do xử lý phải có ít nhất 10 ký tự.');
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
    <Modal open={Boolean(flag)} onClose={onClose} title={`Xác nhận xử lý — flag ${flag.flagNo}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="text-xs text-slate-500 leading-relaxed">
          Chọn hệ quả áp dụng cho giao dịch <b className="font-mono text-slate-900">{flag.txnRef}</b> của user <b>{flag.userName}</b>. Hành động được ghi audit log và không thể hoàn tác từ UI.
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
            <div className="text-sm font-semibold text-slate-900">Khóa tài khoản + hủy giao dịch đang chờ</div>
            <div className="text-xs text-slate-500 mt-0.5 leading-normal">User không đăng nhập được; workflow đang chạy sẽ bồi hoàn (saga). Giao dịch đã completed giữ nguyên.</div>
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
            <div className="text-sm font-semibold text-slate-900">Khóa tài khoản + tạo lệnh đảo giao dịch đã hoàn tất</div>
            <div className="text-xs text-slate-500 mt-0.5 leading-normal">Tạo reversal cho {flag.txnRef} (bút toán đảo, không xóa sổ cái). Cần quyền risk_officer.</div>
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
            <div className="text-sm font-semibold text-slate-900">Đánh dấu an toàn / Chỉ ghi nhận, không can thiệp</div>
            <div className="text-xs text-slate-500 mt-0.5 leading-normal">Giữ hồ sơ để điều tra thêm, không ảnh hưởng user hoặc giao dịch.</div>
          </div>
        </div>

        {/* Reason Input */}
        <div>
          <label htmlFor="fraud-reason" className="text-xs font-bold text-slate-500 block mb-1.5">Lý do xử lý (bắt buộc, ≥ 10 ký tự)</label>
          <textarea
            id="fraud-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Vd: Mẫu hình smurfing rõ, 6 GD/4 phút tới 6 người nhận mới, user không phản hồi cuộc gọi..."
            className="w-full min-h-[72px] rounded-lg border border-slate-200 p-2.5 text-xs font-sans focus:outline-none focus:ring-2 focus:ring-brand/30"
            required
          />
        </div>

        {/* Footer Buttons */}
        <div className="flex justify-end gap-2.5 pt-2">
          <button type="button" onClick={onClose} className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-xs font-medium text-slate-700 cursor-pointer hover:bg-slate-50">
            Hủy
          </button>
          <button
            type="submit"
            disabled={isSubmitting || reason.trim().length < 10}
            className={`h-9 px-4 rounded-lg border-none text-white text-xs font-semibold cursor-pointer transition-colors shadow-xs ${
              selectedOption === 'mark_only' ? 'bg-emerald-600 hover:bg-emerald-700' : 'bg-rose-600 hover:bg-rose-700'
            } ${isSubmitting || reason.trim().length < 10 ? 'opacity-60 cursor-not-allowed' : ''}`}
          >
            {isSubmitting ? 'Đang xử lý…' : selectedOption === 'mark_only' ? 'Đánh dấu an toàn' : 'Xác nhận xử lý gian lận'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
