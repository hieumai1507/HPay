import { isRequestFinalizedCode } from '@/src/queries';

const parseQRUrl = (url: string): { payload: string; signature: string } | null => {
  try {
    const urlObj = new URL(url);
    if (urlObj.protocol !== 'hpay:') return null;
    const payload = urlObj.searchParams.get('payload');
    const signature = urlObj.searchParams.get('signature');
    if (payload && signature) {
      return { payload, signature };
    }
  } catch {
    try {
      const parsed = JSON.parse(url);
      if (parsed.payload && parsed.signature) {
        return { payload: parsed.payload, signature: parsed.signature };
      }
    } catch {}
  }
  return null;
};

describe('TestEdge_QRUntrustedPayload', () => {
  it('correctly parses valid hpay url format', () => {
    const validUrl = 'hpay://qr?payload=abc&signature=xyz';
    const parsed = parseQRUrl(validUrl);
    expect(parsed).toEqual({ payload: 'abc', signature: 'xyz' });
  });

  it('correctly parses valid raw JSON format', () => {
    const validJson = JSON.stringify({ payload: 'def', signature: '123' });
    const parsed = parseQRUrl(validJson);
    expect(parsed).toEqual({ payload: 'def', signature: '123' });
  });

  it('rejects untrusted/malformed payloads', () => {
    expect(parseQRUrl('hpay://qr?unrelated=true')).toBeNull();
    expect(parseQRUrl('https://google.com/qr?payload=abc&signature=xyz')).toBeNull();
    expect(parseQRUrl('invalid-string')).toBeNull();
  });
});

describe('TestEdge_E19_PayExpiredRequest_SafetyCheck', () => {
  it('identifies finalized error codes to stop client pay flows safely', () => {
    expect(isRequestFinalizedCode('ERR_REQUEST_NOT_PAYABLE')).toBe(true);
    expect(isRequestFinalizedCode('ERR_REQUEST_EXPIRED')).toBe(true);
    expect(isRequestFinalizedCode('ERR_REQUEST_NOT_FOUND')).toBe(true);
    expect(isRequestFinalizedCode('ERR_INSUFFICIENT_BALANCE')).toBe(false);
  });
});
