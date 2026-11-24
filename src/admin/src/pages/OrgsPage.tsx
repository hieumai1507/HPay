import { useEffect, useMemo, useState } from 'react';

import { useOrgPolicies, useOrgs, useSaveOrgPolicy, type OrgPolicy, type OrgPolicyTier } from '@/queries';
import { Badge, ErrorNotice } from '@/ui';
import { ACTION_PERMISSION, NO_PERMISSION_REASON } from '@/lib/access';
import { formatVnd } from '@/lib/money';
import { useAdminStore } from '@/stores/admin';

/** "From 10,000,000 ₫ to 40,000,000 ₫" or "From 10,000,000 ₫ with no upper limit". */
function rangeText(p: OrgPolicy): string {
  return p.maxAmount === null
    ? `From ${formatVnd(p.minAmount)} with no upper limit`
    : `From ${formatVnd(p.minAmount)} to ${formatVnd(p.maxAmount)}`;
}

export default function OrgsPage() {
  const { data, isLoading, error } = useOrgs();
  const orgs = useMemo(() => data?.rows ?? [], [data]);
  const canEdit = useAdminStore((s) => s.can(ACTION_PERMISSION.orgPolicy));

  const [selectedOrgId, setSelectedOrgId] = useState<string>('');
  const [search, setSearch] = useState('');
  const selectedOrg = orgs.find((o) => o.org_id === selectedOrgId) ?? orgs[0];

  const policiesQ = useOrgPolicies(selectedOrg?.org_id);
  const policies = useMemo(() => policiesQ.data ?? [], [policiesQ.data]);
  const activePolicies = policies.filter((p) => p.isActive);
  const olderPolicies = policies.filter((p) => !p.isActive);

  // Editing works on a copy of the version the operator picked; saving creates the next version.
  const [editingId, setEditingId] = useState<string | null>(null);
  const [tiers, setTiers] = useState<OrgPolicyTier[]>([]);
  const editing = policies.find((p) => p.id === editingId) ?? null;
  const saveMutation = useSaveOrgPolicy();
  const [message, setMessage] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);

  // A different organization starts with nothing selected for editing and no old message.
  const orgKey = selectedOrg?.org_id;
  useEffect(() => {
    setEditingId(null);
    setTiers([]);
    setMessage(null);
  }, [orgKey]);

  const startEdit = (p: OrgPolicy) => {
    setEditingId(p.id);
    setTiers(p.tiers.map((t) => ({ ...t })));
    setMessage(null);
  };
  const cancelEdit = () => {
    setEditingId(null);
    setTiers([]);
  };
  const changeApprovals = (idx: number, delta: number) =>
    setTiers((prev) =>
      prev.map((t, i) => (i === idx ? { ...t, required_approvals: Math.max(1, t.required_approvals + delta) } : t)),
    );
  const addLevel = () =>
    setTiers((prev) => [...prev, { level: (prev[prev.length - 1]?.level ?? 0) + 1, required_approvals: 1 }]);
  const removeLevel = (idx: number) =>
    setTiers((prev) => prev.filter((_, i) => i !== idx).map((t, i) => ({ ...t, level: i + 1 })));

  const nextVersion = editing ? Math.max(...policies.filter((p) => p.name === editing.name).map((p) => p.version)) + 1 : 0;

  const save = async () => {
    if (!selectedOrg || !editing || saveMutation.isPending || tiers.length === 0) return;
    try {
      const res = await saveMutation.mutateAsync({
        orgId: selectedOrg.org_id,
        name: editing.name,
        minAmount: editing.minAmount,
        maxAmount: editing.maxAmount,
        tiers,
      });
      setMessage({ kind: 'ok', text: `Saved version ${res.version} of "${editing.name}" for ${selectedOrg.name}.` });
      cancelEdit();
    } catch (e: unknown) {
      setMessage({ kind: 'error', text: e instanceof Error ? e.message : String(e) });
    }
  };

  const filteredOrgs = orgs.filter((o) => o.name.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h2 className="text-lg font-bold text-slate-900">
          {selectedOrg ? `${selectedOrg.name} — Approval policy` : 'Organizations & Policy'}
        </h2>
      </div>

      <ErrorNotice error={error} what="organizations" />
      {!canEdit && <div className="text-xs text-slate-500">{NO_PERMISSION_REASON} (orgs.edit)</div>}

      {isLoading ? (
        <div className="p-10 text-center text-slate-500">Loading...</div>
      ) : orgs.length === 0 ? (
        <div className="p-10 text-center text-slate-500">No organizations yet.</div>
      ) : (
        <div className="flex gap-5 flex-col md:flex-row">
          <div className="w-full md:w-72 flex flex-col gap-3 shrink-0">
            <input
              type="text"
              aria-label="Search organizations"
              placeholder="Search organizations…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
            />
            <div className="flex flex-col gap-2.5">
              {filteredOrgs.map((org) => {
                const isSelected = org.org_id === (selectedOrg?.org_id ?? '');
                const initials = org.name
                  .split(' ')
                  .slice(0, 3)
                  .map((w) => w[0])
                  .join('')
                  .toUpperCase();
                return (
                  <button
                    type="button"
                    key={org.org_id}
                    onClick={() => setSelectedOrgId(org.org_id)}
                    className={`text-left bg-white rounded-xl p-3 flex items-center gap-3 cursor-pointer shadow-xs border transition-all ${
                      isSelected ? 'border-brand border-2' : 'border-slate-200'
                    }`}
                  >
                    <div className="w-10 h-10 rounded-lg text-white flex items-center justify-center text-xs font-bold bg-slate-700">
                      {initials}
                    </div>
                    <div>
                      <div className="text-sm font-semibold text-slate-900">{org.name}</div>
                      <div className="text-xs text-slate-500 mt-0.5">
                        {org.memberCount} members · {org.policiesCount} active {org.policiesCount === 1 ? 'policy' : 'policies'}
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {selectedOrg && (
            <div className="flex-1 min-w-0 flex flex-col gap-3.5">
              {selectedOrg.pendingApprovalsCount > 0 && (
                <div className="bg-amber-50 border-l-4 border-amber-500 p-3 px-4 rounded-lg text-xs text-slate-700">
                  <b>{selectedOrg.pendingApprovalsCount} orders awaiting approval</b> use the policy that was active when they were
                  created. A new version applies only to orders created after you save it.
                </div>
              )}

              {message && (
                <div
                  role={message.kind === 'error' ? 'alert' : 'status'}
                  className={`p-3 px-4 rounded-lg text-xs border ${
                    message.kind === 'ok' ? 'bg-emerald-50 border-emerald-200 text-emerald-800' : 'bg-rose-50 border-rose-200 text-rose-800'
                  }`}
                >
                  {message.kind === 'ok' ? message.text : `Could not save: ${message.text}`}
                </div>
              )}

              <ErrorNotice error={policiesQ.error} what="the approval policies" />
              {policiesQ.isLoading && <div className="p-6 text-center text-slate-500 text-sm">Loading policies...</div>}
              {!policiesQ.isLoading && !policiesQ.error && policies.length === 0 && (
                <div className="bg-white rounded-xl p-6 border border-slate-200 text-center text-sm text-slate-500">
                  This organization has no approval policy yet, so its members cannot send disbursements.
                </div>
              )}

              {activePolicies.map((p) => {
                const isEditing = editing?.id === p.id;
                const shown = isEditing ? tiers : p.tiers;
                return (
                  <div key={p.id} className="bg-white rounded-xl p-4 border border-slate-200 shadow-xs">
                    <div className="flex justify-between items-center gap-3 mb-3.5 flex-wrap">
                      <div>
                        <h3 className="text-base font-bold text-slate-900 mb-0.5">
                          {p.name} (version {p.version})
                        </h3>
                        <div className="text-xs text-slate-500">{rangeText(p)}</div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge tone="success">Active</Badge>
                        {!isEditing && (
                          <button
                            type="button"
                            onClick={() => startEdit(p)}
                            disabled={!canEdit}
                            title={canEdit ? undefined : NO_PERMISSION_REASON}
                            className="h-8 px-3 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50 disabled:opacity-60 disabled:cursor-not-allowed"
                          >
                            Edit
                          </button>
                        )}
                      </div>
                    </div>

                    <div className="flex flex-col gap-2.5">
                      {shown.length === 0 && <div className="text-xs text-slate-500">No approval levels.</div>}
                      {shown.map((tier, idx) => (
                        <div key={tier.level} className="flex items-center gap-3.5 bg-slate-50 rounded-xl p-3 flex-wrap">
                          <Badge tone="info">Level {tier.level}</Badge>
                          <span className="text-xs font-medium text-slate-900 flex-1 min-w-40">
                            Needs <b>{tier.required_approvals}</b> {tier.required_approvals === 1 ? 'signature' : 'signatures'} from a level {tier.level} checker
                          </span>
                          {isEditing && (
                            <div className="flex items-center gap-1.5">
                              <div className="flex items-center gap-1.5 border border-slate-200 rounded-md px-1.5 py-0.5 bg-white">
                                <button
                                  type="button"
                                  aria-label={`Fewer signatures for level ${tier.level}`}
                                  onClick={() => changeApprovals(idx, -1)}
                                  className="text-sm font-bold cursor-pointer px-1 text-slate-700 hover:text-brand"
                                >
                                  −
                                </button>
                                <span className="text-xs font-bold min-w-4 text-center">{tier.required_approvals}</span>
                                <button
                                  type="button"
                                  aria-label={`More signatures for level ${tier.level}`}
                                  onClick={() => changeApprovals(idx, 1)}
                                  className="text-sm font-bold cursor-pointer px-1 text-slate-700 hover:text-brand"
                                >
                                  +
                                </button>
                              </div>
                              {tiers.length > 1 && (
                                <button
                                  type="button"
                                  aria-label={`Remove level ${tier.level}`}
                                  onClick={() => removeLevel(idx)}
                                  className="text-xs font-semibold text-rose-600 cursor-pointer hover:underline"
                                >
                                  Remove
                                </button>
                              )}
                            </div>
                          )}
                        </div>
                      ))}

                      {isEditing && (
                        <>
                          <button
                            type="button"
                            onClick={addLevel}
                            className="h-10 rounded-lg border-1.5 border-dashed border-slate-300 bg-white text-slate-700 text-xs font-semibold cursor-pointer hover:border-brand hover:text-brand flex items-center justify-center transition-colors"
                          >
                            + Add level
                          </button>
                          <div className="flex justify-end gap-2.5 mt-1">
                            <button
                              type="button"
                              onClick={cancelEdit}
                              className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50"
                            >
                              Discard changes
                            </button>
                            <button
                              type="button"
                              onClick={save}
                              disabled={saveMutation.isPending || !canEdit || tiers.length === 0}
                              title={canEdit ? undefined : NO_PERMISSION_REASON}
                              className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 shadow-xs disabled:opacity-60 disabled:cursor-not-allowed"
                            >
                              {saveMutation.isPending ? 'Saving…' : `Save as version ${nextVersion}`}
                            </button>
                          </div>
                        </>
                      )}
                    </div>
                  </div>
                );
              })}

              {olderPolicies.length > 0 && (
                <details className="bg-white rounded-xl p-4 border border-slate-200">
                  <summary className="text-xs font-bold text-slate-600 cursor-pointer">
                    Earlier versions ({olderPolicies.length})
                  </summary>
                  <div className="flex flex-col gap-2 mt-3">
                    {olderPolicies.map((p) => (
                      <div key={p.id} className="flex items-center justify-between gap-3 text-xs text-slate-600 bg-slate-50 rounded-lg p-2.5 flex-wrap">
                        <span>
                          <b>{p.name}</b> version {p.version} · {rangeText(p)}
                        </span>
                        <span>{p.tiers.map((t) => `L${t.level}×${t.required_approvals}`).join(' · ') || 'no levels'}</span>
                        <Badge tone="neutral">Replaced</Badge>
                      </div>
                    ))}
                  </div>
                </details>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
