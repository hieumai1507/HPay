import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen } from '@testing-library/react-native';

import SplitScreen from '../split';

jest.mock('expo-router', () => {
  const router = { replace: jest.fn(), push: jest.fn(), back: jest.fn() };
  return { router, useRouter: () => router };
});
jest.mock('@/src/queries', () => ({
  useCreateSplit: () => ({ mutateAsync: jest.fn(), isPending: false }),
}));
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: { children: React.ReactNode }) => children,
}));

function renderScreen() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <SplitScreen />
    </QueryClientProvider>,
  );
}

// TC-USD split screen: a total smaller than the number of people must not crash the screen.
describe('Split bill screen with tiny totals (cents, ADR-0019)', () => {
  it('does not crash while typing 0.01 with two people (1 cent cannot be split two ways)', () => {
    renderScreen();
    const total = screen.getByPlaceholderText('0');
    expect(() => {
      fireEvent.changeText(total, '0');
      fireEvent.changeText(total, '0.');
      fireEvent.changeText(total, '0.0');
      fireEvent.changeText(total, '0.01');
    }).not.toThrow();
    expect(screen.getByPlaceholderText('0')).toBeTruthy();
  });

  it('shows the even shares once the total covers one cent per person', () => {
    renderScreen();
    fireEvent.changeText(screen.getByPlaceholderText('0'), '0.02');
    expect(screen.getAllByText('$0.01').length).toBeGreaterThanOrEqual(2);
  });

  it('does not crash when a third person is added to a $0.02 total', () => {
    renderScreen();
    fireEvent.changeText(screen.getByPlaceholderText('0'), '0.02');
    expect(() => fireEvent.press(screen.getByText(/Add (another )?person/i))).not.toThrow();
  });

  it('explains why submit is refused when the total is smaller than one cent per person', () => {
    renderScreen();
    fireEvent.changeText(screen.getByPlaceholderText('0'), '0.01');
    fireEvent.changeText(screen.getAllByPlaceholderText(/Phone number of person/)[0], '0912345678');
    fireEvent.changeText(screen.getAllByPlaceholderText(/Phone number of person/)[1], '0912345679');
    fireEvent.press(screen.getByText('Create split bill'));
    expect(screen.getByText('The total must be at least $0.01 for each person')).toBeTruthy();
  });
});