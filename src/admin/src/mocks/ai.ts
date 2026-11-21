// Demo data. Only loaded via a dynamic import guarded by
// `import.meta.env.DEV && MOCKS_ENABLED` (VITE_USE_MOCKS=true).
import type { AIKey, AISetting } from '@/queries/useAIQueries';

export const MOCK_AI_KEYS: AIKey[] = [
  { id: 'key-1-claude-prod', name: 'Claude 3.5 Sonnet Prod Key', provider: 'anthropic', api_key: 'sk-ant-api03-••••••••••••••••', is_active: true, created_at: '2026-07-01T08:00:00Z' },
  { id: 'key-2-gpt4o-backup', name: 'GPT-4o Backup Key', provider: 'openai', api_key: 'sk-proj-••••••••••••••••', is_active: true, created_at: '2026-07-05T10:30:00Z' },
];

export const MOCK_AI_SETTINGS: AISetting[] = [
  { function_name: 'kyc_scoring', provider: 'anthropic', model_name: 'claude-3-5-sonnet-20241022', key_id: 'key-1-claude-prod' },
  { function_name: 'fraud_advising', provider: 'openai', model_name: 'gpt-4o-mini', key_id: 'key-2-gpt4o-backup' },
];
