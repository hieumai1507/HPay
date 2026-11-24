import { render, screen } from '@testing-library/react-native';

import { SplitAvatarStack } from '../SplitAvatarStack';

describe('SplitAvatarStack (docs ui-design-spec.html §8, mockup screen 29)', () => {
  const participants = [
    { id: '1', name: 'Minh Anh', paid: true },
    { id: '2', name: 'Thảo Linh', paid: true },
    { id: '3', name: 'Quốc Bảo', paid: false },
  ];

  it('shows collected/total amount and paid count', async () => {
    await render(
      <SplitAvatarStack participants={participants} collectedAmount={420_000n} totalAmount={630_000n} />
    );
    expect(screen.getByText('Đã thu 420.000 ₫ / 630.000 ₫')).toBeTruthy();
    expect(screen.getByText('2/3 người đã trả')).toBeTruthy();
  });

  it('renders initials for every participant', async () => {
    await render(
      <SplitAvatarStack participants={participants} collectedAmount={0n} totalAmount={630_000n} />
    );
    expect(screen.getByText('MA')).toBeTruthy();
    expect(screen.getByText('TL')).toBeTruthy();
    expect(screen.getByText('QB')).toBeTruthy();
  });

  it('dims the avatar of an unpaid participant (opacity .45)', async () => {
    await render(
      <SplitAvatarStack participants={participants} collectedAmount={420_000n} totalAmount={630_000n} />
    );
    const unpaidAvatar = screen.getByTestId('split-avatar-3');
    const paidAvatar = screen.getByTestId('split-avatar-1');
    expect(unpaidAvatar.props.style.opacity).toBe(0.45);
    expect(paidAvatar.props.style.opacity).toBe(1);
  });

  it('shows hours remaining when expiresAt is given', async () => {
    const inTwoHours = new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString();
    await render(
      <SplitAvatarStack
        participants={participants}
        collectedAmount={420_000n}
        totalAmount={630_000n}
        expiresAt={inTwoHours}
      />
    );
    expect(screen.getByText('2/3 người đã trả · còn 2 giờ')).toBeTruthy();
  });
});
