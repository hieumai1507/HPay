import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { idempotencyHeaders } from '@/lib/idempotency';
import { type AdminQueryParams, type Paged, buildQueryString, decodeJsonColumn } from './common';

export interface WorkflowStep {
  name: string;
  status: 'done' | 'fail' | 'active' | 'wait' | 'running';
  durationMs?: number;
  attempt?: number;
  maxAttempts?: number;
  output?: Record<string, unknown> | string | null;
  error?: string | null;
  /** The unedited error text, for the tooltip. */
  rawError?: string | null;
}

export interface WorkflowInstance {
  id: string;
  definitionId: string;
  txnRef?: string;
  amount?: number;
  currentStep: string;
  age: string;
  status: 'running' | 'waiting' | 'failed' | 'completed' | 'compensated' | 'compensation_failed';
  isStuck?: boolean;
  createdAt: string;
  steps?: WorkflowStep[];
  context?: Record<string, unknown>;
}

interface BackendWorkflowRow {
  id: string;
  definition_name?: string;
  definition_version?: number;
  definition_id?: string;
  txn_ref?: string;
  txn_amount?: number;
  current_step: string;
  status: string;
  created_at: string;
  updated_at?: string;
  context?: Record<string, unknown> | string;
}

export function useWorkflowInstances(filters?: AdminQueryParams & { stuckOnly?: boolean; status?: string; definition?: string }, enabled = true) {
  // Server-side filters (docs plan workflow-instances-filters): status,
  // definition, and stuck_only now all run on the full table, not just the
  // current page — map the UI's camelCase/`all` conventions to what the API
  // expects (`all` means "no filter", so it's simply omitted).
  const serverParams = {
    ...filters,
    status: filters?.status && filters.status !== 'all' ? filters.status : undefined,
    definition: filters?.definition && filters.definition !== 'all' ? filters.definition : undefined,
    stuck_only: filters?.stuckOnly ? 'true' : undefined,
    stuckOnly: undefined,
  };

  return useQuery({
    queryKey: ['admin', 'workflows', filters],
    enabled,
    queryFn: async (): Promise<Paged<WorkflowInstance>> => {
      // Errors propagate so the page shows the real failure instead of an empty list.
      const res = await apiRequest<{ workflows: BackendWorkflowRow[] | null; total?: number }>(
        `/v1/admin/workflows${buildQueryString(serverParams)}`
      );
      const items: WorkflowInstance[] = (res?.workflows ?? []).map((w) => {
        const ageMs = Date.now() - new Date(w.created_at).getTime();
        const ageMin = Math.floor(ageMs / 60000);
        return {
          id: w.id,
          definitionId: w.definition_name ? `${w.definition_name} v${w.definition_version}` : (w.definition_id || 'workflow'),
          txnRef: w.txn_ref || '—',
          amount: w.txn_amount,
          currentStep: w.current_step,
          age: ageMin > 0 ? `${ageMin}m` : 'just now',
          status: (w.status as WorkflowInstance['status']) || 'running',
          isStuck: w.status === 'running' && ageMin > 5,
          createdAt: w.created_at,
          context: decodeJsonColumn<Record<string, unknown>>(w.context, {}),
        };
      });
      return { rows: items, total: res?.total ?? items.length };
    },
    refetchInterval: 10_000,
  });
}

interface BackendStepRow {
  step_name: string;
  attempt?: number;
  status: string;
  output?: unknown;
  error?: string | null;
  started_at: string;
  finished_at?: string | null;
}

/** Operators read these, so turn raw database wording into a plain sentence. The raw text stays in `title`. */
export function friendlyStepError(raw: string | null | undefined): string | null {
  if (!raw) return null;
  if (/no rows in result set/i.test(raw)) return 'The record this step needed was not found.';
  if (/context (canceled|deadline exceeded)/i.test(raw)) return 'The step took too long and was stopped.';
  if (/connection (refused|reset)/i.test(raw)) return 'A service this step calls was not reachable.';
  return raw;
}

export function useWorkflowDetail(id: string) {
  return useQuery({
    queryKey: ['admin', 'workflow', id],
    queryFn: async () => {
      const res = await apiRequest<{ instance: BackendWorkflowRow; steps: BackendStepRow[] | null }>(`/v1/admin/workflows/${id}`);
      const inst = res.instance;
      const steps: WorkflowStep[] = (res.steps ?? []).map((s) => {
        const started = Date.parse(s.started_at);
        const finished = s.finished_at ? Date.parse(s.finished_at) : NaN;
        return {
          name: s.step_name,
          status: s.status === 'completed' ? 'done' : s.status === 'failed' ? 'fail' : 'running',
          attempt: s.attempt,
          durationMs: Number.isFinite(started) && Number.isFinite(finished) ? Math.max(0, finished - started) : undefined,
          output: decodeJsonColumn<Record<string, unknown> | null>(s.output, null),
          error: friendlyStepError(s.error),
          rawError: s.error ?? null,
        };
      });
      const createdMs = Date.parse(inst.created_at);
      const ageMin = Number.isFinite(createdMs) ? Math.floor((Date.now() - createdMs) / 60000) : 0;
      return {
        id: inst.id,
        definitionId: inst.definition_name ? `${inst.definition_name} v${inst.definition_version}` : 'workflow',
        txnRef: inst.txn_ref || '—',
        amount: inst.txn_amount,
        currentStep: inst.current_step,
        age: ageMin > 0 ? `${ageMin}m` : 'just now',
        status: (inst.status as WorkflowInstance['status']) || 'running',
        createdAt: inst.created_at,
        steps,
        context: decodeJsonColumn<Record<string, unknown>>(inst.context, {}),
      };
    },
    enabled: !!id,
  });
}
export function useDecideWorkflow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, action, reason }: { id: string; action: 'retry' | 'compensate'; reason: string }) => {
      await apiRequest(`/v1/admin/approvals/${id}/decide`, {
        method: 'POST',
        body: {
          action: action === 'retry' ? 'approve' : 'reject',
          reason,
        },
        headers: idempotencyHeaders(),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'workflows'] });
      qc.invalidateQueries({ queryKey: ['admin', 'workflow'] });
    },
  });
}
