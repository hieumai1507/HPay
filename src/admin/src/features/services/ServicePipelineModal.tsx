import { useState } from 'react';
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

  if (!service) return null;

  const actions: ActionStepConfig[] = service.actions && service.actions.length > 0
    ? service.actions
    : [
        {
          code: 'gl_posting',
          name: 'Hạch toán Sổ cái kép (GL Posting)',
          type: 'GL Posting',
          entry_point: 'model:transdefinition',
          method: 'performAction',
          entry_conditions: 'errorCode == 0',
          required: true,
          params: [
            { name: 'ServiceId', source: 'transaction', field: 'SERVICEID' },
            { name: 'TransRefId', source: 'transaction', field: 'TRANSREFID' },
            { name: 'TransAmount', source: 'transaction', field: 'TOTALAMOUNT' },
          ],
          output: [
            { name: 'data', source: 'metadata', field: 'data', need_secured: false, map_to_txn: 'outputMessage.ACTION' },
          ],
        },
        {
          code: 'bank_gateway_notify',
          name: 'Gọi Gateway Tích hợp Ngân hàng (Directional Integration)',
          type: 'Directional Integration',
          entry_point: 'https://api.bankpartner.vn/v1/p2p-notify',
          method: 'POST',
          entry_conditions: 'transaction.amount >= 1000000',
          required: false,
          params: [
            { name: 'citizenId', source: 'transaction', field: 'TRANSBODY.CITIZENID' },
          ],
          output: [
            { name: 'partnerRef', source: 'response', field: 'partnerRefId', need_secured: true, map_to_txn: 'partnerRefId' },
          ],
          error_mapping: [
            { code: 'err', value: '0', mapping: '0', message: 'Success' },
            { code: 'err', value: '400', mapping: '400', message: 'Bad request partner' },
          ],
        },
      ];

  const inputFields: InputFieldRule[] = service.inputFields && service.inputFields.length > 0
    ? service.inputFields
    : [
        { order: 1, name: 'SERVICEID', rule: 'mapping', type: 'string', required: true },
        { order: 2, name: 'AMOUNT', rule: 'mapping', type: 'float', min_length: 1, max_length: 20, required: true },
        { order: 3, name: 'CURRENCY', rule: 'fixed', type: 'string', regex: '^(VND)$', required: true },
        { order: 4, name: 'SENDERPHONE', rule: 'mapping', type: 'string', min_length: 9, max_length: 11, required: true },
      ];

  const currentAction = actions[selectedActionIndex] || actions[0]!;

  return (
    <Modal open={open} onClose={onClose} title={`Transaction Design Manager · ${service.code}`}>
      <div className="flex flex-col w-[840px] max-w-[90vw]">
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
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Mã Dịch vụ (Code)</label>
                <div className="h-9 flex items-center px-3 bg-slate-100 rounded-lg text-xs font-bold font-mono text-slate-900">{service.code}</div>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Tên Dịch vụ</label>
                <div className="h-9 flex items-center px-3 bg-slate-100 rounded-lg text-xs font-bold text-slate-900">{service.name}</div>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Tên đầy đủ (Full Name)</label>
                <input type="text" defaultValue={service.fullName || service.name} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white" />
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Loại Giao dịch (Transaction Type)</label>
                <select defaultValue={service.transactionType || 'Normal Transaction'} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white">
                  <option value="Normal Transaction">Normal Transaction (Giao dịch thông thường)</option>
                  <option value="Refund">Refund (Hoàn tiền)</option>
                  <option value="Inquiry">Inquiry (Tra cứu)</option>
                  <option value="Reversal">Reversal (Bút toán đảo)</option>
                </select>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Thứ tự Ưu tiên (Action Priority)</label>
                <select defaultValue={service.actionPriority} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white">
                  <option value="before_gl">Before: Call 3rd party first then Post to GL</option>
                  <option value="after_gl">After: Post to GL first then call 3rd party</option>
                </select>
              </div>
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block">Xác nhận Hạch toán (GL Posting Confirmation)</label>
                <div className="flex items-center gap-2.5 h-9">
                  <input type="checkbox" defaultChecked={service.confirmation !== false} id="gl-check" className="rounded border-slate-300 text-brand" />
                  <label htmlFor="gl-check" className="text-xs font-semibold text-slate-800">Tạo bút toán Kế toán kép Sổ cái</label>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 1: Trans Definition (GL Posting) */}
        {activeTab === 'definition' && (
          <div className="flex flex-col gap-3 min-h-[300px]">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">CẤU HÌNH BÚT TOÁN SỔ CÁI KÉP (DEBIT / CREDIT LEDGER RULES)</div>
            <table className="w-full border-collapse text-xs bg-white rounded-lg overflow-hidden border border-slate-200">
              <thead>
                <tr className="bg-slate-50 border-b border-slate-200 text-slate-600">
                  <th className="p-2.5 text-left font-bold">Vế Hạch toán</th>
                  <th className="p-2.5 text-left font-bold">Chủ tài khoản (Owner Type)</th>
                  <th className="p-2.5 text-left font-bold">Loại Tài khoản (Type)</th>
                  <th className="p-2.5 text-left font-bold">Quy tắc tính Số tiền (Amount Formula)</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                <tr>
                  <td className="p-2.5"><Badge tone="danger">DEBIT (Nợ)</Badge></td>
                  <td className="p-2.5 font-mono">USER_MAIN_WALLET</td>
                  <td className="p-2.5 text-slate-700">Asset / Primary Pocket</td>
                  <td className="p-2.5 font-mono">AMOUNT + FEE</td>
                </tr>
                <tr>
                  <td className="p-2.5"><Badge tone="success">CREDIT (Có)</Badge></td>
                  <td className="p-2.5 font-mono">RECEIVER_MAIN_WALLET</td>
                  <td className="p-2.5 text-slate-700">Liability / Primary Pocket</td>
                  <td className="p-2.5 font-mono">AMOUNT</td>
                </tr>
                <tr>
                  <td className="p-2.5"><Badge tone="info">CREDIT (Có Phí)</Badge></td>
                  <td className="p-2.5 font-mono">SYS_FEE_ACCOUNT</td>
                  <td className="p-2.5 text-slate-700">Income / Fee Pool</td>
                  <td className="p-2.5 font-mono">FEE</td>
                </tr>
              </tbody>
            </table>
          </div>
        )}

        {/* Tab 2: Validations */}
        {activeTab === 'validation' && (
          <div className="flex flex-col gap-3 min-h-[300px]">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">TRƯỜNG DỮ LIỆU ĐẦU VÀO &amp; QUY TẮC VALIDATION</div>
            <table className="w-full border-collapse text-xs bg-white rounded-lg overflow-hidden border border-slate-200">
              <thead>
                <tr className="bg-slate-50 border-b border-slate-200 text-slate-600">
                  <th className="p-2.5 text-left font-bold">Order</th>
                  <th className="p-2.5 text-left font-bold">Field Name</th>
                  <th className="p-2.5 text-left font-bold">Rule</th>
                  <th className="p-2.5 text-left font-bold">Type</th>
                  <th className="p-2.5 text-left font-bold">Min / Max Length</th>
                  <th className="p-2.5 text-left font-bold">Regex Pattern</th>
                  <th className="p-2.5 text-left font-bold">Bắt buộc</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {inputFields.map((f) => (
                  <tr key={f.name}>
                    <td className="p-2.5 font-bold text-slate-700">#{f.order}</td>
                    <td className="p-2.5 font-mono font-semibold text-slate-900">{f.name}</td>
                    <td className="p-2.5"><Badge tone={f.rule === 'mapping' ? 'info' : 'warning'}>{f.rule}</Badge></td>
                    <td className="p-2.5 text-slate-700">{f.type}</td>
                    <td className="p-2.5 text-slate-700">{f.min_length || 0} - {f.max_length || '∞'}</td>
                    <td className="p-2.5 font-mono text-slate-600">{f.regex || '—'}</td>
                    <td className="p-2.5">{f.required ? <span className="text-emerald-600 font-bold">✓ Có</span> : 'Không'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Tab 3: Action Pipeline */}
        {activeTab === 'pipeline' && (
          <div className="flex gap-4 min-h-[300px]">
            <div className="w-64 flex flex-col gap-2.5 shrink-0">
              <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">CHUỖI ACTION STEPS</div>
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
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-[11px] font-bold text-slate-500 mb-1 block">Entry Point (Endpoint)</label>
                  <input type="text" defaultValue={currentAction.entry_point} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white" />
                </div>
                <div>
                  <label className="text-[11px] font-bold text-slate-500 mb-1 block">Method</label>
                  <input type="text" defaultValue={currentAction.method} className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white" />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 4: Offer Auth Matrix */}
        {activeTab === 'offer' && (
          <div className="flex flex-col gap-3 min-h-[300px]">
            <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">MA TRẬN HẠN MỨC &amp; PHƯƠNG THỨC XÁC THỰC (AUTH MATRIX)</div>
            <table className="w-full border-collapse text-xs bg-white rounded-lg overflow-hidden border border-slate-200">
              <thead>
                <tr className="bg-slate-50 border-b border-slate-200 text-slate-600">
                  <th className="p-2.5 text-left font-bold">Ngưỡng Tiền Từ (VND)</th>
                  <th className="p-2.5 text-left font-bold">Đến Ngưỡng (VND)</th>
                  <th className="p-2.5 text-left font-bold">Phương thức Auth yêu cầu</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                <tr>
                  <td className="p-2.5 font-medium text-slate-900">0 đ</td>
                  <td className="p-2.5 font-medium text-slate-900">2.000.000 đ</td>
                  <td className="p-2.5"><Badge tone="info">PIN (Mã PIN Ví)</Badge></td>
                </tr>
                <tr>
                  <td className="p-2.5 font-medium text-slate-900">2.000.000 đ</td>
                  <td className="p-2.5 font-medium text-slate-900">50.000.000 đ</td>
                  <td className="p-2.5"><Badge tone="warning">OTP (Mã SMS/Smart OTP)</Badge></td>
                </tr>
                <tr>
                  <td className="p-2.5 font-medium text-slate-900">50.000.000 đ</td>
                  <td className="p-2.5 font-medium text-slate-900">Không giới hạn</td>
                  <td className="p-2.5"><Badge tone="danger">2FA (Xác thực Sinh trắc học / FIDO2)</Badge></td>
                </tr>
              </tbody>
            </table>
          </div>
        )}

        {/* Footer */}
        <div className="flex justify-end gap-2.5 mt-4 pt-3 border-t border-slate-200">
          <button type="button" onClick={onClose} className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50">Đóng</button>
          <button
            type="button"
            onClick={() => {
              alert(`Đã lưu thiết kế Transaction Design cho Dịch vụ ${service.code}!`);
              onClose();
            }}
            className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
          >
            Lưu Transaction Design
          </button>
        </div>
      </div>
    </Modal>
  );
}
