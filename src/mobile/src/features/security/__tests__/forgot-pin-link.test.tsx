import { fireEvent, render, screen } from '@testing-library/react-native';
import { Pressable, Text } from 'react-native';

import { PinPrompt } from '../components/PinPrompt';
import { usePin } from '../usePin';

const mockPush = jest.fn();
jest.mock('expo-router', () => ({ useRouter: () => ({ push: mockPush }) }));

describe('PinPrompt Forgot PIN? link', () => {
  it('is hidden when no onForgot is given (existing callers unchanged)', () => {
    render(<PinPrompt visible onSubmit={jest.fn()} onCancel={jest.fn()} />);
    expect(screen.queryByText('Forgot PIN?')).toBeNull();
  });

  it('calls onForgot when given', () => {
    const onForgot = jest.fn();
    render(<PinPrompt visible onSubmit={jest.fn()} onCancel={jest.fn()} onForgot={onForgot} />);
    fireEvent.press(screen.getByLabelText('pin-forgot'));
    expect(onForgot).toHaveBeenCalled();
  });
});

describe('usePin wires Forgot PIN? to the reset flow', () => {
  let result: Promise<string | null> | null = null;
  function Harness() {
    const { requestPin, pinPrompt } = usePin();
    return (
      <>
        <Pressable accessibilityLabel="ask" onPress={() => { result = requestPin(); }}>
          <Text>ask</Text>
        </Pressable>
        {pinPrompt}
      </>
    );
  }

  it('cancels the pending PIN request and opens reset-pin', async () => {
    render(<Harness />);
    fireEvent.press(screen.getByLabelText('ask'));
    fireEvent.press(screen.getByLabelText('pin-forgot'));
    await expect(result).resolves.toBeNull();
    expect(mockPush).toHaveBeenCalledWith('/(main)/security/reset-pin');
  });
});