import { render, screen } from '@testing-library/react-native';

import { StatusBadge } from '../StatusBadge';

describe('StatusBadge status->color mapping (ui-design-spec.html §2.1)', () => {
  it.each([
    ['completed', 'Hoàn tất', 'color-emerald-700'],
    ['processing', 'Đang xử lý', 'color-cyan-700'],
    ['running', 'Đang xử lý', 'color-cyan-700'],
    ['pending_approval', 'Chờ duyệt', 'color-amber-700'],
    ['manual_review', 'Đang xét duyệt', 'color-amber-700'],
    ['failed', 'Thất bại', 'color-rose-700'],
    ['rejected', 'Bị từ chối', 'color-rose-700'],
    ['expired', 'Đã hết hạn', 'color-rose-700'],
    ['created', 'Đã tạo', 'color-slate-700'],
    ['reversed', 'Đã hoàn tiền', 'color-slate-700'],
    ['compensated', 'Đã hoàn tiền', 'color-slate-700'],
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
