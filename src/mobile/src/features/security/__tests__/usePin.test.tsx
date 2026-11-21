import { act, fireEvent, render, screen } from '@testing-library/react-native';
import { Pressable, Text } from 'react-native';

import { usePin } from '../usePin';

let result: Promise<string | null> | null = null;

function Harness() {
  const { requestPin, pinPrompt } = usePin();
  return (
    <>
      <Pressable
        accessibilityLabel="ask"
        onPress={() => {
          result = requestPin();
        }}
      >
        <Text>ask</Text>
      </Pressable>
      {pinPrompt}
    </>
  );
}

describe('Test_usePin_CollectsPinFromUser', () => {
  beforeEach(() => {
    result = null;
  });

  it('resolves with exactly the digits the user typed', async () => {
    await render(<Harness />);
    await fireEvent.press(screen.getByLabelText('ask'));
    await fireEvent.changeText(screen.getByLabelText('OTP digit 1'), '482913');
    await fireEvent.press(screen.getByLabelText('pin-confirm'));
    await expect(result).resolves.toBe('482913');
  });

  it('does not submit until 6 digits are entered', async () => {
    await render(<Harness />);
    await fireEvent.press(screen.getByLabelText('ask'));
    await fireEvent.changeText(screen.getByLabelText('OTP digit 1'), '4829');
    const confirm = screen.getByLabelText('pin-confirm');
    expect(confirm.props.accessibilityState?.disabled).toBe(true);
  });

  it('resolves null when the user cancels', async () => {
    await render(<Harness />);
    await fireEvent.press(screen.getByLabelText('ask'));
    await fireEvent.press(screen.getByLabelText('pin-cancel'));
    await expect(result).resolves.toBeNull();
  });

  it('never supplies a PIN by itself', async () => {
    await render(<Harness />);
    await fireEvent.press(screen.getByLabelText('ask'));
    let settled = false;
    void result!.then(() => {
      settled = true;
    });
    await act(async () => {
      await Promise.resolve();
    });
    expect(settled).toBe(false);
  });
});
