import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useOfferDetail, useSaveOffer, type OfferServiceConfig, type AuthMatrixRule } from '@/queries';
import { Badge } from '@/ui';

export default function OfferDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: offer, isLoading } = useOfferDetail(id);
  const saveOfferMutation = useSaveOffer();

  const [selectedServiceCode, setSelectedServiceCode] = useState('P2P_TRANSFER');

  // Local state for editable service limits & auth matrix
  const [maxPerTxn, setMaxPerTxn] = useState(100000000);
  const [dailyLimit, setDailyLimit] = useState(500000000);
  const [monthlyLimit, setMonthlyLimit] = useState(2000000000);
  const [intervalSec, setIntervalSec] = useState(3);

  const [authMatrix, setAuthMatrix] = useState<AuthMatrixRule[]>([
    { minAmount: 0, maxAmount: 2000000, authMethod: 'pin' },
    { minAmount: 2000000, maxAmount: 50000000, authMethod: 'otp' },
    { minAmount: 50000000, authMethod: '2fa' },
  ]);

  if (isLoading || !offer) {
    return <div className="p-10 text-center text-slate-500">Đang tải cấu hình Offer…</div>;
  }

  const handleSave = async () => {
    try {
      await saveOfferMutation.mutateAsync({
        ...offer,
        services: (offer.services || []).map((s) =>
          s.serviceCode === selectedServiceCode
            ? {
                ...s,
                maxAmountPerTxn: maxPerTxn,
                dailyLimit,
                monthlyLimit,
                intervalSec,
                authMatrix,
              }
            : s,
        ),
      });
      alert(`Đã lưu cấu hình Offer ${offer.name} thành công!`);
    } catch (e: unknown) {
      alert(`Lỗi: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  const handleUpdateAuthMethod = (index: number, newMethod: AuthMatrixRule['authMethod']) => {
    setAuthMatrix((prev) =>
      prev.map((item, idx) => (idx === index ? { ...item, authMethod: newMethod } : item)),
    );
  };

  const servicesList: OfferServiceConfig[] = offer.services.length > 0
    ? offer.services
    : [
        {
          serviceId: 'srv-1',
          serviceCode: 'P2P_TRANSFER',
          serviceName: 'Chuyển tiền P2P Ví-đến-Ví',
          isEnabled: true,
          maxAmountPerTxn: 100000000,
          dailyLimit: 500000000,
          monthlyLimit: 2000000000,
          intervalSec: 3,
          authMatrix,
        },
        {
          serviceId: 'srv-2',
          serviceCode: 'MONEY_REQUEST',
          serviceName: 'Xin tiền & Split Bill',
          isEnabled: true,
          maxAmountPerTxn: 20000000,
          dailyLimit: 100000000,
          monthlyLimit: 500000000,
          intervalSec: 2,
          authMatrix: [{ minAmount: 0, authMethod: 'pin' }],
        },
        {
          serviceId: 'srv-3',
          serviceCode: 'TOPUP_BANK',
          serviceName: 'Nạp tiền Ngân hàng',
          isEnabled: true,
          maxAmountPerTxn: 50000000,
          dailyLimit: 200000000,
          monthlyLimit: 1000000000,
          intervalSec: 5,
          authMatrix: [{ minAmount: 0, authMethod: 'otp' }],
        },
      ];

  const activeService = servicesList.find((s) => s.serviceCode === selectedServiceCode) || servicesList[0]!;

  return (
    <div className="flex flex-col gap-4">
      {/* Topbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-3 flex-wrap">
          <button
            type="button"
            onClick={() => navigate('/offers')}
            className="px-3 py-1.5 rounded-lg border border-slate-200 bg-white text-xs font-semibold text-slate-700 cursor-pointer hover:bg-slate-50"
          >
            ← Danh sách Offer
          </button>
          <h2 className="text-base font-bold text-slate-900">Gói: {offer.name}</h2>
          <Badge tone={offer.isActive ? 'success' : 'neutral'}>
            {offer.isActive ? 'Active' : 'Draft'}
          </Badge>
        </div>

        <button
          type="button"
          onClick={handleSave}
          disabled={saveOfferMutation.isPending}
          className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
        >
          {saveOfferMutation.isPending ? 'Đang lưu…' : 'Lưu cấu hình Offer'}
        </button>
      </div>

      {/* Main Layout */}
      <div className="flex gap-5 flex-col md:flex-row">
        {/* Left Column: Authorized Services */}
        <div className="w-full md:w-70 flex flex-col gap-3 shrink-0">
          <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">DỊCH VỤ CẤP PHÉP (AUTHORIZED SERVICES)</div>

          <div className="flex flex-col gap-2.5">
            {servicesList.map((srv) => {
              const isSelected = srv.serviceCode === activeService.serviceCode;
              return (
                <div
                  key={srv.serviceCode}
                  onClick={() => {
                    setSelectedServiceCode(srv.serviceCode);
                    setMaxPerTxn(srv.maxAmountPerTxn);
                    setDailyLimit(srv.dailyLimit);
                    setMonthlyLimit(srv.monthlyLimit);
                    setIntervalSec(srv.intervalSec);
                    if (srv.authMatrix) setAuthMatrix(srv.authMatrix);
                  }}
                  className={`bg-white rounded-xl p-3 cursor-pointer shadow-xs border transition-all ${
                    isSelected ? 'border-brand border-2' : 'border-slate-200'
                  }`}
                >
                  <div className="text-sm font-semibold text-slate-900">{srv.serviceName}</div>
                  <div className="text-xs text-slate-500 font-mono mt-0.5">{srv.serviceCode} · Configured</div>
                </div>
              );
            })}

            <button
              type="button"
              onClick={() => alert('Thêm Dịch vụ mới vào danh sách cấp phép của Offer này')}
              className="h-10 rounded-lg border-1.5 border-dashed border-slate-300 bg-white text-slate-700 text-xs font-semibold cursor-pointer hover:border-brand hover:text-brand flex items-center justify-center transition-colors"
            >
              + Thêm Dịch vụ vào Offer
            </button>
          </div>
        </div>

        {/* Right Column: Limits & Auth Matrix Builder */}
        <div className="flex-1 flex flex-col gap-4">
          {/* Card 1: Limits */}
          <div className="bg-white rounded-xl p-4.5 border border-slate-200 shadow-xs">
            <h3 className="text-sm font-bold text-slate-900 mb-3.5">
              Cấu hình Hạn mức Dịch vụ · <span className="font-mono">{activeService.serviceCode}</span>
            </h3>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">HẠN MỨC MỖI GIAO DỊCH (MAX PER TXN)</label>
                <input
                  type="number"
                  value={maxPerTxn}
                  onChange={(e) => setMaxPerTxn(Number(e.target.value))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{(maxPerTxn / 1000000).toFixed(0)} Triệu ₫</span>
              </div>

              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">HẠN MỨC NGÀY (DAILY LIMIT)</label>
                <input
                  type="number"
                  value={dailyLimit}
                  onChange={(e) => setDailyLimit(Number(e.target.value))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{(dailyLimit / 1000000).toFixed(0)} Triệu ₫</span>
              </div>

              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">HẠN MỨC THÁNG (MONTHLY LIMIT)</label>
                <input
                  type="number"
                  value={monthlyLimit}
                  onChange={(e) => setMonthlyLimit(Number(e.target.value))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{(monthlyLimit / 1000000000).toFixed(1)} Tỷ ₫</span>
              </div>

              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">GIÂY GIỮA 2 GIAO DỊCH (INTERVAL SEC)</label>
                <input
                  type="number"
                  value={intervalSec}
                  onChange={(e) => setIntervalSec(Number(e.target.value))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{intervalSec} giây</span>
              </div>
            </div>
          </div>

          {/* Card 2: Auth Matrix */}
          <div className="bg-white rounded-xl p-4.5 border border-slate-200 shadow-xs">
            <h3 className="text-sm font-bold text-slate-900 mb-1">Ma trận Xác thực (Authentication Matrix)</h3>
            <div className="text-xs text-slate-500 mb-3.5">
              Yêu cầu xác thực tương ứng theo ngưỡng số tiền của giao dịch.
            </div>

            <div className="flex flex-col gap-2.5">
              {authMatrix.map((item, idx) => (
                <div key={idx} className="flex items-center gap-3.5 bg-slate-50 rounded-xl p-3">
                  <Badge tone={idx === 0 ? 'info' : idx === 1 ? 'warning' : 'danger'}>
                    {idx === 0 ? 'Mức nhỏ' : idx === 1 ? 'Mức vừa' : 'Mức lớn'}
                  </Badge>

                  <span className="text-xs text-slate-800 flex-1">
                    {item.maxAmount
                      ? `Từ ${item.minAmount.toLocaleString('vi-VN')} ₫ đến ${item.maxAmount.toLocaleString('vi-VN')} ₫`
                      : `Trên ${item.minAmount.toLocaleString('vi-VN')} ₫`}{' '}
                    → Bắt buộc <b>{item.authMethod.toUpperCase()}</b>
                  </span>

                  <select
                    value={item.authMethod}
                    onChange={(e) => handleUpdateAuthMethod(idx, e.target.value as AuthMatrixRule['authMethod'])}
                    className="h-8 px-2.5 rounded-md border border-slate-200 text-xs font-semibold text-slate-900 bg-white cursor-pointer"
                  >
                    <option value="none">NONE (Không xác thực)</option>
                    <option value="pin">PIN (Mã PIN / Biometric)</option>
                    <option value="otp">OTP (Mã OTP SMS/Push)</option>
                    <option value="2fa">2FA (Cả PIN + OTP)</option>
                  </select>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
