import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({
  approvals: { rows: [] as unknown[], total: 0 },
  workflows: { rows: [] as unknown[], total: 0 },
}));

vi.mock('@/queries', () => ({
  useAttentionSummary: () => ({ data: { total: state.approvals.total + state.workflows.total } }),
  useNeedsManualApprovals: () => ({ data: state.approvals, isLoading: false, error: null }),
  useWorkflowInstances: () => ({ data: state.workflows, isLoading: false, error: null }),
}));

import NeedsAttentionPage from './NeedsAttentionPage';

function renderPage() {
  return render(
    <MemoryRouter>
      <NeedsAttentionPage />
    </MemoryRouter>,
  );
}

describe('NeedsAttentionPage', () => {
  afterEach(() => {
    cleanup();
    state.approvals = { rows: [], total: 0 };
    state.workflows = { rows: [], total: 0 };
  });

  it('shows an empty state when nothing is parked', () => {
    renderPage();
    expect(screen.getByRole('status').textContent).toMatch(/Nothing needs attention/);
  });

  it('lists both kinds with ref code, reason and a detail link', () => {
    state.approvals = {
      rows: [
        { id: 'a1', subjectType: 'reversal', refCode: 'MP123', amount: 400000, reason: 'receiver balance too low', parkedAt: '2026-11-24T00:00:00Z' },
      ],
      total: 1,
    };
    state.workflows = {
      rows: [{ id: 'w1', definitionId: 'p2p_transfer v1', txnRef: 'MP999', amount: 1000, currentStep: 'CREDIT_RECEIVER', createdAt: '2026-11-24T00:00:00Z' }],
      total: 1,
    };
    renderPage();
    expect(screen.getByText('MP123')).toBeTruthy();
    expect(screen.getByText('receiver balance too low')).toBeTruthy();
    expect(screen.getByText('MP999')).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Detail' }).getAttribute('href')).toBe('/workflows/w1');
    expect(screen.queryByRole('status')).toBeNull();
  });
});
