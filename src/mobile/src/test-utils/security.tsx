import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, fireEvent, render, screen } from '@testing-library/react-native';
import type { ReactElement } from 'react';

export function renderWithClient(ui: ReactElement) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

/** Types a 6-digit value into the OTPInput whose boxes are labelled `${label} 1..6`. */
export function typeDigits(label: string, value: string) {
  fireEvent.changeText(screen.getByLabelText(`${label} 1`), value);
}

/** A promise whose resolution the test controls (to hold a request in flight). */
export function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}
/** Press and let the resulting async state updates settle inside act. */
export async function pressAsync(element: Parameters<typeof fireEvent.press>[0]) {
  await act(async () => {
    fireEvent.press(element);
  });
}
