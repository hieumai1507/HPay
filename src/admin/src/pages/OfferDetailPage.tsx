import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useOfferDetail, type OfferServiceConfig, type AuthMatrixRule } from '@/queries';
import { ACTION_PERMISSION, NO_PERMISSION_REASON } from '@/lib/access';
import { formatVnd, moneyErrorMessage, parseMinorUnits, type MoneyParseResult } from '@/lib/money';
import { useAdminStore } from '@/stores/admin';
import { Badge, ErrorNotice, NotAvailableNote } from '@/ui';

const MAX_INTERVAL_SEC = 86_400n;

/** Shown only when the offer has no configured services: all-zero, inert placeholder. */
const EMPTY_SERVICE: OfferServiceConfig = {
  serviceId: '',
  serviceCode: '—',
  serviceName: '—',
  isEnabled: false,
  maxAmountPerTxn: 0,
  dailyLimit: 0,
  monthlyLimit: 0,
  intervalSec: 0,
  authMatrix: [],
};

function hint(r: MoneyParseResult): string {
  return r.ok ? formatVnd(r.value) : moneyErrorMessage(r.reason);
}

export default function OfferDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: offer, isLoading, error } = useOfferDetail(id);
  // UX only: the backend does not enforce admin roles yet (audit BE-H6).
  const canConfigure = useAdminStore((s) => s.can(ACTION_PERMISSION.offerConfig));

  const [selectedServiceCode, setSelectedServiceCode] = useState<string | null>(null);

  // Edits are kept as the raw text the operator typed and are validated on every
  // render (digits only, no negatives, upper bound). Nothing goes through Number().
  // undefined = untouched, so the field shows the real value from the API.
  const [draft, setDraft] = useState<{
    max?: string;
    daily?: string;
    monthly?: string;
    interval?: string;
    auth?: AuthMatrixRule[];
  }>({});

  if (error) {
    return <ErrorNotice error={error} what="the offer" />;
  }

  if (isLoading || !offer) {
    return <div className="p-10 text-center text-slate-500">Loading...</div>;
  }

  const handleUpdateAuthMethod = (index: number, newMethod: AuthMatrixRule['authMethod']) => {
    setDraft((d) => ({ ...d, auth: authMatrix.map((item, idx) => (idx === index ? { ...item, authMethod: newMethod } : item)) }));
  };

  const servicesList: OfferServiceConfig[] = offer.services;
  const activeService =
    servicesList.find((s) => s.serviceCode === selectedServiceCode) ?? servicesList[0] ?? EMPTY_SERVICE;
  const hasServices = servicesList.length > 0;

  const maxPerTxn = draft.max ?? String(activeService.maxAmountPerTxn);
  const dailyLimit = draft.daily ?? String(activeService.dailyLimit);
  const monthlyLimit = draft.monthly ?? String(activeService.monthlyLimit);
  const intervalSec = draft.interval ?? String(activeService.intervalSec);
  const authMatrix: AuthMatrixRule[] = draft.auth ?? activeService.authMatrix ?? [];

  const maxPerTxnR = parseMinorUnits(maxPerTxn);
  const dailyR = parseMinorUnits(dailyLimit);
  const monthlyR = parseMinorUnits(monthlyLimit);
  const intervalR = parseMinorUnits(intervalSec, MAX_INTERVAL_SEC);

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
            ← Back to Plans
          </button>
          <h2 className="text-base font-bold text-slate-900">Plan: {offer.name}</h2>
          <Badge tone={offer.isActive ? 'success' : 'neutral'}>
            {offer.isActive ? 'Active' : 'Draft'}
          </Badge>
        </div>

        <button
          type="button"
          disabled
          title={canConfigure ? 'Not available yet: the backend has no write endpoint for offers (GET only).' : NO_PERMISSION_REASON}
          className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
        >
          Save Plan settings
        </button>
      </div>

      <NotAvailableNote>{'the backend has no endpoint to save offers yet (only GET /v1/admin/offers and /v1/admin/offers/{id}). Values below are validated previews and cannot be saved.'}</NotAvailableNote>
      {!hasServices && <div className="text-xs text-slate-500">This offer has no configured services.</div>}

      {/* Main Layout */}
      <div className="flex gap-5 flex-col md:flex-row">
        {/* Left Column: Authorized Services */}
        <div className="w-full md:w-70 flex flex-col gap-3 shrink-0">
          <div className="text-[11px] font-bold text-slate-500 tracking-wider uppercase">AUTHORIZED SERVICES</div>

          <div className="flex flex-col gap-2.5">
            {servicesList.map((srv) => {
              const isSelected = srv.serviceCode === activeService.serviceCode;
              return (
                <div
                  key={srv.serviceCode}
                  onClick={() => {
                    setSelectedServiceCode(srv.serviceCode);
                    setDraft({});
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
              disabled
              title="Not available yet: the backend has no write endpoint for offers."
              className="h-10 rounded-lg border-1.5 border-dashed border-slate-300 bg-white text-slate-700 text-xs font-semibold cursor-pointer hover:border-brand hover:text-brand flex items-center justify-center transition-colors"
            >
              + Add service to Plan
            </button>
          </div>
        </div>

        {/* Right Column: Limits & Auth Matrix Builder */}
        <div className="flex-1 flex flex-col gap-4">
          {/* Card 1: Limits */}
          <div className="bg-white rounded-xl p-4.5 border border-slate-200 shadow-xs">
            <h3 className="text-sm font-bold text-slate-900 mb-3.5">
              Service Limit settings · <span className="font-mono">{activeService.serviceCode}</span>
            </h3>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">MAX PER TRANSACTION</label>
                <input
                  type="text"
                  inputMode="numeric"
                  disabled={!hasServices}
                  aria-invalid={!maxPerTxnR.ok}
                  value={maxPerTxn}
                  onChange={(e) => setDraft((d) => ({ ...d, max: e.target.value }))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{hint(maxPerTxnR)}</span>
              </div>

              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">DAILY LIMIT</label>
                <input
                  type="text"
                  inputMode="numeric"
                  disabled={!hasServices}
                  aria-invalid={!dailyR.ok}
                  value={dailyLimit}
                  onChange={(e) => setDraft((d) => ({ ...d, daily: e.target.value }))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{hint(dailyR)}</span>
              </div>

              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">MONTHLY LIMIT</label>
                <input
                  type="text"
                  inputMode="numeric"
                  disabled={!hasServices}
                  aria-invalid={!monthlyR.ok}
                  value={monthlyLimit}
                  onChange={(e) => setDraft((d) => ({ ...d, monthly: e.target.value }))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{hint(monthlyR)}</span>
              </div>

              <div>
                <label className="text-[11px] font-bold text-slate-500 mb-1 block uppercase">SECONDS BETWEEN TRANSACTIONS</label>
                <input
                  type="text"
                  inputMode="numeric"
                  disabled={!hasServices}
                  aria-invalid={!intervalR.ok}
                  value={intervalSec}
                  onChange={(e) => setDraft((d) => ({ ...d, interval: e.target.value }))}
                  className="w-full h-9 px-3 rounded-lg border border-slate-200 text-sm font-mono font-semibold bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                />
                <span className="text-[11px] text-slate-500 mt-0.5 block">{intervalR.ok ? `${intervalR.value}s` : moneyErrorMessage(intervalR.reason)}</span>
              </div>
            </div>
          </div>

          {/* Card 2: Auth Matrix */}
          <div className="bg-white rounded-xl p-4.5 border border-slate-200 shadow-xs">
            <h3 className="text-sm font-bold text-slate-900 mb-1">Authentication matrix</h3>
            <div className="text-xs text-slate-500 mb-3.5">
              Required authentication by Transaction Amount threshold.
            </div>

            <div className="flex flex-col gap-2.5">
              {authMatrix.map((item, idx) => (
                <div key={idx} className="flex items-center gap-3.5 bg-slate-50 rounded-xl p-3">
                  <Badge tone={idx === 0 ? 'info' : idx === 1 ? 'warning' : 'danger'}>
                    {idx === 0 ? 'Small Amount' : idx === 1 ? 'Medium Amount' : 'Large Amount'}
                  </Badge>

                  <span className="text-xs text-slate-800 flex-1">
                    {item.maxAmount
                      ? `From ${item.minAmount.toLocaleString('en-US')} ₫ to ${item.maxAmount.toLocaleString('en-US')} ₫`
                      : `Above ${item.minAmount.toLocaleString('en-US')} ₫`}{' '}
                    → Requires <b>{item.authMethod.toUpperCase()}</b>
                  </span>

                  <select
                    value={item.authMethod}
                    onChange={(e) => handleUpdateAuthMethod(idx, e.target.value as AuthMatrixRule['authMethod'])}
                    className="h-8 px-2.5 rounded-md border border-slate-200 text-xs font-semibold text-slate-900 bg-white cursor-pointer"
                  >
                    <option value="none">NONE (No authentication)</option>
                    <option value="pin">PIN (PIN / Biometric)</option>
                    <option value="otp">OTP (verification code via SMS/Push)</option>
                    <option value="2fa">2FA (PIN + verification code)</option>
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
