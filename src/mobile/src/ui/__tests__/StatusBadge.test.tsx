import { render, screen } from '@testing-library/react-native';

import { StatusBadge } from '../StatusBadge';

describe('StatusBadge status->color mapping (ui-design-spec.html §2.1)', () => {
  it.each([
    ['completed', 'Completed', 'color-emerald-700'],
    ['processing', 'Processing', 'color-cyan-700'],
    ['running', 'Processing', 'color-cyan-700'],
    ['pending_approval', 'Pending', 'color-amber-700'],
    ['manual_review', 'Under review', 'color-amber-700'],
    ['failed', 'Failed', 'color-rose-700'],
    ['rejected', 'Rejected', 'color-rose-700'],
    ['expired', 'Expired', 'color-rose-700'],
    ['created', 'Created', 'color-slate-700'],
    ['reversed', 'Refunded', 'color-slate-700'],
    ['compensated', 'Refunded', 'color-slate-700'],
  ] as const)('renders %s with the spec-mandated label and class %s', async (status, labelText, expectedClass) => {
    await render(<StatusBadge status={status} />);
    const label = screen.getByText(labelText);
    expect(label.props.className).toContain(expectedClass);
  });

  it('shows the ↩ icon instead of a dot for reversed/compensated', async () => {
    await render(<StatusBadge status="reversed" />);
    expect(screen.getByText('↩')).toBeTruthy();
  });
});
