import { useState } from 'react';
import { useOrgs, useSaveOrgPolicy, type OrgPolicyTier } from '@/queries';
import { Badge, ErrorNotice } from '@/ui';
import { ACTION_PERMISSION, NO_PERMISSION_REASON } from '@/lib/access';
import { MAX_MINOR_UNITS, formatMillions } from '@/lib/money';
import { useAdminStore } from '@/stores/admin';

export default function OrgsPage() {
  const { data, isLoading, error } = useOrgs();
  const orgs = data?.rows ?? [];
  const savePolicyMutation = useSaveOrgPolicy();
  // UX only: the backend does not enforce admin roles yet (audit BE-H6).
  const canEdit = useAdminStore((s) => s.can(ACTION_PERMISSION.orgPolicy));

  const [selectedOrgId, setSelectedOrgId] = useState<string>('');
  const [search, setSearch] = useState('');

  // Current active selected org
  const selectedOrg = orgs.find((o) => o.org_id === selectedOrgId) || orgs[0];

  // Editable tiers state for the selected org's policy
  const [tiers, setTiers] = useState<OrgPolicyTier[]>([
    { level: 1, min_amount: 10000000, required_approvals: 1 },
    { level: 2, min_amount: 40000000, required_approvals: 1 },
  ]);

  const filteredOrgs = orgs.filter((o) => o.name.toLowerCase().includes(search.toLowerCase()));

  const handleUpdateApproval = (levelIndex: number, delta: number) => {
    setTiers((prev) =>
      prev.map((t, idx) =>
        idx === levelIndex
          ? { ...t, required_approvals: Math.max(1, t.required_approvals + delta) }
          : t,
      ),
    );
  };

  const handleAddLevel = () => {
    setTiers((prev) => {
      // Integer minor units with an upper bound: refuse to grow past it instead of overflowing.
      const next = (prev[prev.length - 1]?.min_amount ?? 10000000) * 2;
      if (!Number.isSafeInteger(next) || BigInt(next) > MAX_MINOR_UNITS) return prev;
      return [...prev, { level: prev.length + 1, min_amount: next, required_approvals: 1 }];
    });
  };

  const handleSave = async () => {
    if (!selectedOrg || savePolicyMutation.isPending) return;
    try {
      const res = await savePolicyMutation.mutateAsync({
        orgId: selectedOrg.org_id,
        name: 'Chi lớn',
        tiers,
      });
      alert(`Đã lưu chính sách phê duyệt mới (version v${res.version}) cho ${selectedOrg.name}!`);
    } catch (e: unknown) {
      alert(`Lỗi: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      {/* Topbar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h2 className="text-lg font-bold text-slate-900">
          {selectedOrg ? `${selectedOrg.name} — Chính sách phê duyệt` : 'Tổ chức & Policy'}
        </h2>
        <button
          type="button"
          onClick={() => alert('Thêm chính sách phê duyệt mới')}
          disabled={!canEdit}
          title={canEdit ? undefined : NO_PERMISSION_REASON}
          className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs"
        >
          + Thêm policy
        </button>
      </div>

      <ErrorNotice error={error} what="organizations" />
      {!canEdit && <div className="text-xs text-slate-500">{NO_PERMISSION_REASON} (orgs.edit)</div>}

      {isLoading ? (
        <div className="p-10 text-center text-slate-500">Đang tải danh sách tổ chức…</div>
      ) : (
        <div className="flex gap-5 flex-col md:flex-row">
          {/* Left Column — Org List Sidebar */}
          <div className="w-full md:w-72 flex flex-col gap-3 shrink-0">
            <input
              type="text"
              placeholder="🔍 Tìm tổ chức…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
            />

            <div className="flex flex-col gap-2.5">
              {filteredOrgs.map((org) => {
                const isSelected = org.org_id === (selectedOrg?.org_id ?? '');
                const initials = org.name.split(' ').slice(0, 3).map((w) => w[0]).join('').toUpperCase();
                return (
                  <div
                    key={org.org_id}
                    onClick={() => setSelectedOrgId(org.org_id)}
                    className={`bg-white rounded-xl p-3 flex items-center gap-3 cursor-pointer shadow-xs border transition-all ${
                      isSelected ? 'border-brand border-2' : 'border-slate-200'
                    }`}
                  >
                    <div
                      className={`w-10 h-10 rounded-lg text-white flex items-center justify-center text-xs font-bold ${
                        isSelected ? 'bg-indigo-900' : 'bg-amber-800'
                      }`}
                    >
                      {initials}
                    </div>
                    <div>
                      <div className="text-sm font-semibold text-slate-900">{org.name}</div>
                      <div className="text-xs text-slate-500 mt-0.5">
                        {org.memberCount} thành viên · {org.policiesCount} policies
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Right Column — Policy Panel */}
          {selectedOrg && (
            <div className="flex-1 flex flex-col gap-3.5">
              {/* In-flight snapshot warning card */}
              {selectedOrg.pendingApprovalsCount > 0 && (
                <div className="bg-amber-50 border-l-4 border-amber-500 p-3 px-4 rounded-lg text-xs text-slate-700">
                  ⚠️ Có <b>{selectedOrg.pendingApprovalsCount} lệnh đang chờ duyệt</b> theo policy snapshot cũ. Thay đổi dưới đây chỉ áp dụng cho lệnh tạo mới.
                </div>
              )}

              {/* Policy Editor Card */}
              <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs">
                <div className="flex justify-between items-center mb-3.5">
                  <div>
                    <h3 className="text-base font-bold text-slate-900 mb-0.5">Chi lớn (v3)</h3>
                    <div className="text-xs text-slate-500">Áp dụng: 10.000.000 ₫ → không giới hạn · đang hiệu lực</div>
                  </div>
                  <Badge tone="success">Active</Badge>
                </div>

                {/* Tiers List */}
                <div className="flex flex-col gap-2.5">
                  {tiers.map((tier, idx) => (
                    <div key={idx} className="flex items-center gap-3.5 bg-slate-50 rounded-xl p-3">
                      <Badge tone="info">Level {tier.level}</Badge>
                      <span className="text-xs font-medium text-slate-900 flex-1">
                        Từ {formatMillions(tier.min_amount)} — cần <b>{tier.required_approvals}</b> chữ ký Checker L{tier.level}
                      </span>
                      <div className="flex items-center gap-1.5 border border-slate-200 rounded-md px-1.5 py-0.5 bg-white">
                        <button
                          type="button"
                          onClick={() => handleUpdateApproval(idx, -1)}
                          disabled={!canEdit}
                          className="border-none bg-none text-sm font-bold cursor-pointer px-1 text-slate-700 hover:text-brand"
                        >
                          −
                        </button>
                        <span className="text-xs font-bold min-w-4 text-center">{tier.required_approvals}</span>
                        <button
                          type="button"
                          onClick={() => handleUpdateApproval(idx, 1)}
                          disabled={!canEdit}
                          className="border-none bg-none text-sm font-bold cursor-pointer px-1 text-slate-700 hover:text-brand"
                        >
                          +
                        </button>
                      </div>
                    </div>
                  ))}

                  <button
                    type="button"
                    onClick={handleAddLevel}
                    disabled={!canEdit}
                    className="h-10 rounded-lg border-1.5 border-dashed border-slate-300 bg-white text-slate-700 text-xs font-semibold cursor-pointer hover:border-brand hover:text-brand flex items-center justify-center transition-colors"
                  >
                    + Thêm level
                  </button>
                </div>
              </div>

              {/* Secondary Static Policy Card */}
              <div className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs">
                <div className="flex justify-between items-center">
                  <div>
                    <h3 className="text-base font-bold text-slate-900 mb-0.5">Chi thường (v1)</h3>
                    <div className="text-xs text-slate-500">Dưới 10.000.000 ₫ · cần 1 chữ ký Checker L1</div>
                  </div>
                  <Badge tone="success">Active</Badge>
                </div>
              </div>

              {/* Save Bar */}
              <div className="flex justify-end gap-2.5 mt-2">
                <button
                  type="button"
                  className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50"
                >
                  Hủy thay đổi
                </button>
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={savePolicyMutation.isPending || !canEdit}
                  title={canEdit ? undefined : NO_PERMISSION_REASON}
                  className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs disabled:opacity-60 disabled:cursor-not-allowed"
                >
                  {savePolicyMutation.isPending ? 'Đang lưu…' : 'Lưu — tạo version v4'}
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
