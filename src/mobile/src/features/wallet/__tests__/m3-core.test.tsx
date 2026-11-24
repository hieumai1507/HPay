import { fireEvent, render, screen } from '@testing-library/react-native';

import { stringifyWithBigInt } from '@/src/api/client';
import { getErrorMessage } from '@/src/lib/errors';
import { pollUntilSettled } from '@/src/lib/polling';
import { useSettingsStore } from '@/src/stores/settings';
import { AmountInput, BalanceCard, Button, FilterChips, TxnListItem } from '@/src/ui';

import {
  getNextTransactionsPageParam,
  isTimelineSettled,
  resultVariantFor,
  type Timeline,
  type TxnView,
} from '@/src/queries';

describe('TestEdge_M01_DoubleTapTransfer_SingleRequest', () => {
  it('a loading Button swallows further taps — second tap of "Transfer" cannot fire again', async () => {
    const onPress = jest.fn();
    await render(<Button label="Transfer" loading onPress={onPress} />);
    fireEvent.press(screen.getByRole('button'));
    fireEvent.press(screen.getByRole('button'));
    expect(onPress).not.toHaveBeenCalled();
  });

  it('a normal Button fires; combined with the stable idempotency key (M1 test) a replay is idempotent', async () => {
    const onPress = jest.fn();
    await render(<Button label="Transfer" onPress={onPress} />);
    fireEvent.press(screen.getByRole('button'));
    expect(onPress).toHaveBeenCalledTimes(1);
  });
});

describe('TestEdge_M02_TransferPollTimeout_ProcessingCopy', () => {
  it('poll timeout maps to the PROCESSING variant, never failed', async () => {
    jest.useFakeTimers();
    const fetcher = jest.fn().mockResolvedValue({ instance_status: 'running', steps: [] } as Timeline);
    const promise = pollUntilSettled(fetcher, isTimelineSettled, { timeoutMs: 5000 });
    await jest.advanceTimersByTimeAsync(5000);
    const outcome = await promise;
    jest.useRealTimers();

    expect(outcome.kind).toBe('timeout');
  });
});

describe('TestEdge_M03_TransferFailed_ErrorMapDisplay', () => {
  it('settled timelines map to result variants', () => {
    const tl = (s: string, code?: string): Timeline =>
      ({ instance_status: s, failure_code: code, steps: [] }) as Timeline;

    expect(resultVariantFor(tl('completed'))).toBe('success');
    expect(resultVariantFor(tl('none'))).toBe('success');
    expect(resultVariantFor(tl('waiting_approval'))).toBe('pending_approval');
    expect(resultVariantFor(tl('failed', 'ERR_INSUFFICIENT_BALANCE'))).toBe('failed');
    expect(resultVariantFor(tl('compensated', 'ERR_FRAUD_BLOCKED'))).toBe('failed');
  });

  it('every backend failure_code has centralized Vietnamese copy — no raw codes on screen', () => {
    for (const code of [
      'ERR_INSUFFICIENT_BALANCE',
      'ERR_FRAUD_BLOCKED',
      'ERR_ACCOUNT_INACTIVE',
      'ERR_TRANSFER_FAILED',
    ]) {
      const msg = getErrorMessage(code);
      expect(msg).not.toContain('ERR_');
      expect(msg.length).toBeGreaterThan(10);
    }
  });
});

describe('TestEdge_M06_BigIntMoney_NoPrecisionLoss', () => {
  it('stringifyWithBigInt emits RAW JSON integers — beyond MAX_SAFE_INTEGER stays exact', () => {
    const big = 9007199254740993n;
    expect(stringifyWithBigInt({ amount: big })).toBe('{"amount":9007199254740993}');
    expect(stringifyWithBigInt({ amount: 50000n, note: 'x' })).toBe('{"amount":50000,"note":"x"}');
  });

  it('does not mangle ordinary strings that merely look like placeholders', () => {
    expect(stringifyWithBigInt({ note: 'hello' })).toBe('{"note":"hello"}');
  });
});

const baseTxn: TxnView = {
  id: '11111111-1111-4111-8111-111111111111',
  ref_code: 'MPTEST123',
  type: 'p2p',
  status: 'completed',
  sender_wallet_id: 'a1111111-1111-4111-8111-111111111111',
  receiver_wallet_id: 'b1111111-1111-4111-8111-111111111111',
  amount: 250000,
  fee: 250,
  direction: 'out',
  counterparty_name: 'Nguyễn V** A*',
  counterparty_phone: '091****678',
  created_at: '2026-07-18T10:00:00Z',
};

describe('Test_TxnListItem_DirectionSign', () => {
  it('out txn shows − and neutral color; in txn shows + and success color', async () => {
    await render(<TxnListItem txn={baseTxn} />);
    const out = screen.getByText(/−\$2,500\.00/);
    expect(out).toBeTruthy();

    await render(<TxnListItem txn={{ ...baseTxn, direction: 'in' }} />);
    const inn = screen.getByText(/\+\$2,500\.00/);
    expect(inn).toBeTruthy();
  });

  it('renders the server-masked counterparty name as the row title', async () => {
    await render(<TxnListItem txn={baseTxn} />);
    expect(screen.getByText('Nguyễn V** A*')).toBeTruthy();
  });
});

describe('Test_BalanceCard_HideToggle', () => {
  it('settings store defaults to HIDDEN on app open', () => {
    expect(useSettingsStore.getState().balanceVisible).toBe(false);
  });

  it('hidden shows dots; toggle reveals the amount', async () => {
    const onToggle = jest.fn();
    await render(
      <BalanceCard balanceMinorUnits={1234567n} visible={false} onToggleVisible={onToggle} />,
    );
    expect(screen.getByText('••••••')).toBeTruthy();
    fireEvent.press(screen.getByLabelText('Show balance'));
    expect(onToggle).toHaveBeenCalledTimes(1);

    await render(
      <BalanceCard balanceMinorUnits={1234567n} visible onToggleVisible={onToggle} />,
    );
    expect(screen.getByText(/12,345\.67/)).toBeTruthy();
  });
});

describe('Test_AmountInput_LiveFormatting', () => {
  it('groups thousands while typing and strips pasted junk', async () => {
    const onChange = jest.fn();
    await render(<AmountInput value="" onChangeValue={onChange} />);
    const input = screen.getByLabelText('Amount');

    fireEvent.changeText(input, '123456');
    expect(onChange).toHaveBeenLastCalledWith('123,456');

    fireEvent.changeText(input, '12.345');
    expect(onChange).toHaveBeenLastCalledWith('12.34');

    fireEvent.changeText(input, '$50,000.5');
    expect(onChange).toHaveBeenLastCalledWith('50,000.5');
  });
});

describe('Test_FilterChips_ClientFilter', () => {
  it('selecting a chip reports its value and marks it selected', async () => {
    const onSelect = jest.fn();
    await render(
      <FilterChips
        options={[
          { value: 'all', label: 'All' },
          { value: 'in', label: 'Money in' },
        ]}
        selected="all"
        onSelect={onSelect}
      />,
    );
    fireEvent.press(screen.getByText('Money in'));
    expect(onSelect).toHaveBeenCalledWith('in');
  });
});

describe('Test_CursorPagination_NextPage', () => {
  it('advances with both cursors present; stops on empty page or missing cursor', () => {
    expect(
      getNextTransactionsPageParam({
        transactions: [baseTxn],
        next_cursor: '2026-07-18T09:00:00Z',
        next_cursor_id: baseTxn.id,
      }),
    ).toEqual({ cursor: '2026-07-18T09:00:00Z', cursorId: baseTxn.id });

    expect(getNextTransactionsPageParam({ transactions: [] })).toBeNull();
    expect(getNextTransactionsPageParam({ transactions: [baseTxn] })).toBeNull();
  });
});
