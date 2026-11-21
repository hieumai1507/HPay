import { useState } from 'react';
import {
  useAIKeys,
  useAISettings,
  useCreateAIKey,
  useUpdateAIKey,
  useUpdateAISetting,
  type AIKey,
  type AISetting,
} from '@/queries';
import { Badge, ErrorNotice, Modal } from '@/ui';
import { ACTION_PERMISSION, NO_PERMISSION_REASON } from '@/lib/access';
import { useAdminStore } from '@/stores/admin';

export default function AISettingsPage() {
  const { data: keys = [], isLoading: loadingKeys, error: keysError } = useAIKeys();
  const { data: settings = [], isLoading: loadingSettings, error: settingsError } = useAISettings();
  // UX only: the backend does not enforce admin roles yet (audit BE-H6).
  const canManage = useAdminStore((s) => s.can(ACTION_PERMISSION.aiSettings));

  const createKeyMutation = useCreateAIKey();
  const updateKeyMutation = useUpdateAIKey();
  const updateSettingMutation = useUpdateAISetting();

  // New key form state
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyProvider, setNewKeyProvider] = useState<'anthropic' | 'openai'>('anthropic');
  const [newKeyValue, setNewKeyValue] = useState('');

  // Confirm disable modal state
  const [confirmKeyDisable, setConfirmKeyDisable] = useState<AIKey | null>(null);

  const handleCreateKey = async (e: React.FormEvent) => {
    e.preventDefault();
    if (createKeyMutation.isPending) return;
    if (!newKeyName.trim() || !newKeyValue.trim()) {
      alert('Vui lòng điền tên và giá trị API key.');
      return;
    }
    try {
      await createKeyMutation.mutateAsync({
        name: newKeyName.trim(),
        provider: newKeyProvider,
        api_key: newKeyValue.trim(),
      });
      setNewKeyName('');
      setNewKeyValue('');
      alert('Đã thêm AI Key mới thành công!');
    } catch (err: unknown) {
      alert(`Lỗi: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  const handleToggleActive = async (key: AIKey) => {
    if (key.is_active) {
      const isUsed = settings.some((s) => s.key_id === key.id);
      if (isUsed) {
        setConfirmKeyDisable(key);
        return;
      }
    }
    await performKeyToggle(key, !key.is_active);
  };

  const performKeyToggle = async (key: AIKey, newActiveState: boolean) => {
    if (updateKeyMutation.isPending) return;
    try {
      await updateKeyMutation.mutateAsync({
        id: key.id,
        name: key.name,
        is_active: newActiveState,
      });
      setConfirmKeyDisable(null);
    } catch (err: unknown) {
      alert(`Lỗi: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  const handleSettingChange = async (
    functionName: 'kyc_scoring' | 'fraud_advising',
    provider: 'anthropic' | 'openai' | 'mock',
    modelName: string,
    keyId: string,
  ) => {
    if (updateSettingMutation.isPending) return;
    try {
      await updateSettingMutation.mutateAsync({
        function_name: functionName,
        provider,
        model_name: modelName,
        key_id: provider === 'mock' ? undefined : keyId,
      });
      alert(`Đã cập nhật cấu hình model routing cho ${functionName}!`);
    } catch (err: unknown) {
      alert(`Lỗi: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  const isLoading = loadingKeys || loadingSettings;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h2 className="text-lg font-bold text-slate-900">Cấu hình AI &amp; Model Routing (F11)</h2>
        <span className="text-xs text-slate-500">Dynamic LLM Provider &amp; Secret Management</span>
      </div>

      <ErrorNotice error={keysError ?? settingsError} what="AI configuration" />
      {!canManage && <div className="text-xs text-slate-500">{NO_PERMISSION_REASON} (ai.manage)</div>}

      {isLoading ? (
        <div className="p-10 text-center text-slate-500">Đang tải AI config…</div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {/* Left Column: AI Keys Management */}
          <div className="flex flex-col gap-4">
            <div className="bg-white rounded-xl p-4.5 border border-slate-200 shadow-xs">
              <h3 className="text-base font-bold text-slate-900 mb-3">Danh sách AI API Keys</h3>
              <div className="flex flex-col gap-2.5">
                {keys.map((k) => {
                  const usedCount = settings.filter((s) => s.key_id === k.id).length;
                  return (
                    <div key={k.id} className="flex items-center justify-between p-3 bg-slate-50 rounded-lg border border-slate-200 gap-3">
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <b className="text-sm text-slate-900">{k.name}</b>
                          <Badge tone={k.provider === 'anthropic' ? 'info' : 'warning'}>
                            {k.provider}
                          </Badge>
                          {usedCount > 0 && (
                            <span className="text-[11px] text-brand font-semibold bg-cyan-50 px-1.5 py-0.5 rounded">
                              Dùng bởi {usedCount} chức năng
                            </span>
                          )}
                        </div>
                        <div className="font-mono text-xs text-slate-500 mt-1">
                          {k.api_key || '••••••••••••••••'}
                        </div>
                      </div>

                      {/* Active Toggle */}
                      <button
                        type="button"
                        onClick={() => handleToggleActive(k)}
                        disabled={!canManage || updateKeyMutation.isPending}
                        title={canManage ? undefined : NO_PERMISSION_REASON}
                        className={`px-2.5 py-1 rounded-full text-white font-bold text-[11px] cursor-pointer transition-colors ${
                          k.is_active ? 'bg-emerald-600' : 'bg-slate-400'
                        }`}
                      >
                        {k.is_active ? 'ACTIVE' : 'INACTIVE'}
                      </button>
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Add New Key Form */}
            <div className="bg-white rounded-xl p-4.5 border border-slate-200 shadow-xs">
              <h3 className="text-base font-bold text-slate-900 mb-3">Thêm API Key mới</h3>
              <form onSubmit={handleCreateKey} className="flex flex-col gap-3">
                <div>
                  <label className="text-xs font-bold text-slate-500 mb-1 block">Tên gợi nhớ (Name)</label>
                  <input
                    type="text"
                    placeholder="Vd: Claude 3.5 Sonnet Prod Key"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                  />
                </div>
                <div>
                  <label className="text-xs font-bold text-slate-500 mb-1 block">Provider</label>
                  <select
                    value={newKeyProvider}
                    onChange={(e) => setNewKeyProvider(e.target.value as 'anthropic' | 'openai')}
                    className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                  >
                    <option value="anthropic">Anthropic (Claude)</option>
                    <option value="openai">OpenAI (ChatGPT)</option>
                  </select>
                </div>
                <div>
                  <label className="text-xs font-bold text-slate-500 mb-1 block">API Key Secret</label>
                  <input
                    type="password"
                    placeholder="sk-ant-... hoặc sk-proj-..."
                    value={newKeyValue}
                    onChange={(e) => setNewKeyValue(e.target.value)}
                    className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
                  />
                </div>
                <button
                  type="submit"
                  disabled={createKeyMutation.isPending || !canManage}
                  title={canManage ? undefined : NO_PERMISSION_REASON}
                  className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 transition-colors shadow-xs"
                >
                  {createKeyMutation.isPending ? 'Đang tạo…' : '+ Lưu Key Mới'}
                </button>
              </form>
            </div>
          </div>

          {/* Right Column: Model Routing Settings */}
          <div className="flex flex-col gap-4">
            <div className="bg-white rounded-xl p-4.5 border border-slate-200 shadow-xs">
              <h3 className="text-base font-bold text-slate-900 mb-1">Cấu hình Model Routing theo Chức Năng</h3>
              <div className="text-xs text-slate-500 mb-4">
                Chuyển đổi provider, model name hoặc key tự động mà không cần restart server backend.
              </div>

              <div className="flex flex-col gap-3.5">
                {(['kyc_scoring', 'fraud_advising'] as const).map((funcName) => {
                  const setting = settings.find((s) => s.function_name === funcName) || {
                    function_name: funcName,
                    provider: 'mock' as const,
                    model_name: 'mock-v1',
                  };
                  return (
                    <SettingRow
                      key={funcName}
                      setting={setting}
                      availableKeys={keys.filter((k) => k.is_active)}
                      onSave={handleSettingChange}
                      isSaving={updateSettingMutation.isPending || !canManage}
                    />
                  );
                })}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Confirm Disable Modal */}
      {confirmKeyDisable && (
        <Modal
          open={Boolean(confirmKeyDisable)}
          onClose={() => setConfirmKeyDisable(null)}
          title="Xác nhận vô hiệu hóa API Key"
        >
          <div className="flex flex-col gap-3.5">
            <div className="text-sm text-slate-700">
              Cảnh báo: Key <b>{confirmKeyDisable.name}</b> đang được liên kết với chức năng AI active. Vô hiệu hóa key này sẽ khiến chức năng đó trả lỗi <code>ERR_KEY_INACTIVE</code> trừ khi bạn chọn key khác!
            </div>
            <div className="flex justify-end gap-2.5 mt-2">
              <button
                type="button"
                onClick={() => setConfirmKeyDisable(null)}
                className="h-9 px-4 rounded-lg border border-slate-200 bg-white text-slate-700 font-semibold text-xs cursor-pointer hover:bg-slate-50"
              >
                Hủy
              </button>
              <button
                type="button"
                onClick={() => performKeyToggle(confirmKeyDisable, false)}
                disabled={updateKeyMutation.isPending}
                className="h-9 px-4 rounded-lg bg-rose-600 text-white font-semibold text-xs cursor-pointer hover:bg-rose-700 shadow-xs"
              >
                Xác nhận vô hiệu hóa
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

function SettingRow({
  setting,
  availableKeys,
  onSave,
  isSaving,
}: {
  setting: AISetting;
  availableKeys: AIKey[];
  onSave: (
    funcName: 'kyc_scoring' | 'fraud_advising',
    provider: 'anthropic' | 'openai' | 'mock',
    modelName: string,
    keyId: string,
  ) => void;
  isSaving: boolean;
}) {
  const [provider, setProvider] = useState(setting.provider);
  const [modelName, setModelName] = useState(setting.model_name);
  const [keyId, setKeyId] = useState(setting.key_id || '');

  const filteredKeys = availableKeys.filter(
    (k) => provider === 'mock' || k.provider === provider,
  );

  return (
    <div className="bg-slate-50 rounded-xl p-3.5 border border-slate-200 flex flex-col gap-3">
      <div className="flex justify-between items-center">
        <b className="text-sm text-slate-900">
          {setting.function_name === 'kyc_scoring' ? '🪪 KYC Scoring (Vision OCR)' : '🚨 Fraud Advising (LLM Advisory)'}
        </b>
        <Badge tone="info">{provider}</Badge>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="text-xs font-bold text-slate-500 mb-1 block">Provider</label>
          <select
            value={provider}
            onChange={(e) => {
              const p = e.target.value as 'anthropic' | 'openai' | 'mock';
              setProvider(p);
              if (p === 'anthropic') setModelName('claude-3-5-sonnet-20241022');
              else if (p === 'openai') setModelName('gpt-4o-mini');
              else setModelName('mock-v1');
            }}
            className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
          >
            <option value="anthropic">Anthropic (Claude)</option>
            <option value="openai">OpenAI (ChatGPT)</option>
            <option value="mock">Mock Engine (Dev / Offline)</option>
          </select>
        </div>

        <div>
          <label className="text-xs font-bold text-slate-500 mb-1 block">Model Name</label>
          <input
            type="text"
            value={modelName}
            onChange={(e) => setModelName(e.target.value)}
            className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
          />
        </div>

        {provider !== 'mock' && (
          <div className="col-span-2">
            <label className="text-xs font-bold text-slate-500 mb-1 block">API Key Bind</label>
            <select
              value={keyId}
              onChange={(e) => setKeyId(e.target.value)}
              className="w-full h-9 px-3 rounded-lg border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-brand/30"
            >
              <option value="">-- Chọn Active Key --</option>
              {filteredKeys.map((k) => (
                <option key={k.id} value={k.id}>
                  {k.name} ({k.provider})
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      <div className="flex justify-end mt-2">
        <button
          type="button"
          disabled={isSaving}
          onClick={() => onSave(setting.function_name, provider, modelName, keyId)}
          className="h-9 px-4 rounded-lg bg-brand text-white font-semibold text-xs cursor-pointer hover:bg-brand-600 transition-colors shadow-xs"
        >
          {isSaving ? 'Đang lưu…' : 'Lưu routing này'}
        </button>
      </div>
    </div>
  );
}
