import { ApiError } from '@/src/lib/errors';

import { isPinNotSet, weakPinReason } from '../pinErrors';

describe('weakPinReason mirrors the server rule', () => {
  it.each(['111111', '000000', '123456', '654321', '012345', '12345', '1234567', '12ab56', ''])(
    'blocks %p',
    (pin) => {
      expect(weakPinReason(pin)).not.toBeNull();
    },
  );
  it.each(['482913', '135790', '123457', '100000'])('accepts %p', (pin) => {
    expect(weakPinReason(pin)).toBeNull();
  });
});

describe('isPinNotSet', () => {
  it('is true only for ERR_PIN_NOT_SET', () => {
    expect(isPinNotSet(new ApiError('ERR_PIN_NOT_SET', 403))).toBe(true);
    expect(isPinNotSet(new ApiError('ERR_PIN_INCORRECT', 403))).toBe(false);
    expect(isPinNotSet(new Error('ERR_PIN_NOT_SET'))).toBe(false);
    expect(isPinNotSet(null)).toBe(false);
  });
});