import { describe, expect, it } from 'vitest';

import { formatAge } from './age';

const NOW = Date.parse('2026-11-24T12:00:00Z');

describe('formatAge', () => {
  it('shows minutes under an hour', () => {
    expect(formatAge('2026-11-24T11:30:00Z', NOW)).toBe('30m');
  });
  it('shows hours under a day', () => {
    expect(formatAge('2026-11-24T07:00:00Z', NOW)).toBe('5h');
  });
  it('shows days from 24 hours', () => {
    expect(formatAge('2026-11-22T12:00:00Z', NOW)).toBe('2d');
  });
  it('clamps a future or invalid timestamp to 0m', () => {
    expect(formatAge('2026-11-24T13:00:00Z', NOW)).toBe('0m');
    expect(formatAge('not-a-date', NOW)).toBe('0m');
  });
});