import { buildTimeline } from '../order-status';
import type { ApprovalDetail } from '@/src/queries';

const base: ApprovalDetail = {
  id: 'r1',
  status: 'pending',
  current_level: 1,
  org_id: 'o1',
  org_name: 'Test Org',
  ref_code: 'OD-0001',
  amount: 1_000_000,
  fee: 0,
  maker_id: 'm1',
  maker_name: 'Maker',
  created_at: '2026-07-08T09:12:00Z',
  expires_at: new Date(Date.now() + 22 * 60 * 60 * 1000).toISOString(),
  policy_name: 'default',
  signatures_required: 2,
  signatures: [],
};

describe('buildTimeline (mockup 03-business.html frame 19)', () => {
  it('shows created step + waiting step when no one has signed yet', () => {
    const steps = buildTimeline(base);
    expect(steps[0]).toMatchObject({ title: 'Bạn đã tạo lệnh', dot: 'done' });
    expect(steps[1]).toMatchObject({ dot: 'active' });
    expect(steps[1].title).toContain('chờ 2 chữ ký');
    expect(steps[2]).toMatchObject({ title: 'Thực hiện chuyển tiền', dot: 'wait' });
  });

  it('marks a recorded approval as done and shows remaining signatures on the active step', () => {
    const d: ApprovalDetail = {
      ...base,
      current_level: 2,
      signatures: [
        { checker_id: 'c1', checker_name: 'Trần Quang', level: 1, action: 'approve', acted_at: '2026-07-08T09:26:00Z' },
      ],
    };
    const steps = buildTimeline(d);
    expect(steps[1]).toMatchObject({ title: 'Level 1 — Trần Quang đã duyệt', dot: 'done' });
    expect(steps[2].title).toBe('Level 2 — chờ 1 chữ ký');
    expect(steps[2].dot).toBe('active');
  });

  it('stops the timeline at a rejection and does not show a waiting step after it', () => {
    const d: ApprovalDetail = {
      ...base,
      status: 'rejected',
      signatures: [
        { checker_id: 'c1', checker_name: 'Trần Quang', level: 1, action: 'reject', acted_at: '2026-07-08T09:26:00Z' },
      ],
    };
    const steps = buildTimeline(d);
    expect(steps).toHaveLength(2);
    expect(steps[1]).toMatchObject({ title: 'Level 1 — Trần Quang đã từ chối', dot: 'rejected' });
  });

  it('shows a final done step when approved', () => {
    const d: ApprovalDetail = {
      ...base,
      status: 'approved',
      signatures: [
        { checker_id: 'c1', checker_name: 'Trần Quang', level: 1, action: 'approve', acted_at: '2026-07-08T09:26:00Z' },
      ],
    };
    const steps = buildTimeline(d);
    expect(steps[steps.length - 1]).toMatchObject({ title: 'Đã thực hiện chuyển tiền', dot: 'done' });
  });

  it('shows an expired terminal step', () => {
    const d: ApprovalDetail = { ...base, status: 'expired' };
    const steps = buildTimeline(d);
    expect(steps[steps.length - 1].dot).toBe('rejected');
  });

  it('shows a cancelled terminal step', () => {
    const d: ApprovalDetail = { ...base, status: 'cancelled' };
    const steps = buildTimeline(d);
    expect(steps[steps.length - 1]).toMatchObject({ title: 'Bạn đã hủy lệnh', dot: 'rejected' });
  });
});
