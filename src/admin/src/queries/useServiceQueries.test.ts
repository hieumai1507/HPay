import { describe, expect, it } from 'vitest';

import { decodeJsonColumn, mapServiceRow, type ServiceRow } from './useServiceQueries';

// The API sends Postgres jsonb as base64 text ("W10=" is "[]"). A page that treated that string as an
// array crashed with "actions.map is not a function" (service catalog, Action Pipeline modal).
const b64 = (v: unknown) => btoa(JSON.stringify(v));

describe('decodeJsonColumn', () => {
  it('decodes base64 JSON written by the Go API', () => {
    expect(decodeJsonColumn('W10=', ['x'])).toEqual([]);
    expect(decodeJsonColumn('e30=', { a: 1 })).toEqual({});
    expect(decodeJsonColumn(b64([{ code: 'gl_posting' }]), [])).toEqual([{ code: 'gl_posting' }]);
  });

  it('keeps UTF-8 text intact', () => {
    expect(decodeJsonColumn(btoa(String.fromCharCode(...new TextEncoder().encode('["Hũ"]'))), [])).toEqual(['Hũ']);
  });

  it('accepts a value that is already parsed or plain JSON text', () => {
    expect(decodeJsonColumn([1, 2], [])).toEqual([1, 2]);
    expect(decodeJsonColumn('[1,2]', [])).toEqual([1, 2]);
  });

  it('falls back instead of throwing on null, empty or garbage', () => {
    expect(decodeJsonColumn(null, ['d'])).toEqual(['d']);
    expect(decodeJsonColumn(undefined, ['d'])).toEqual(['d']);
    expect(decodeJsonColumn('', ['d'])).toEqual(['d']);
    expect(decodeJsonColumn('%%%not base64 or json%%%', ['d'])).toEqual(['d']);
  });
});

describe('mapServiceRow', () => {
  const row: ServiceRow = {
    id: 'a0000000-0000-0000-0000-000000000002',
    code: 'MONEY_REQUEST',
    name: 'Request money & split bill',
    full_name: 'Request Money & Split Bill Service',
    service_note: null,
    category: 'payment',
    transaction_type: 'Normal Transaction',
    action_priority: 'before_gl',
    status: 'active',
    confirmation: true,
    service_target: 'request_money',
    channels: b64(['Mobile']),
    amount_formula: 'e30=',
    actions: 'W10=',
    input_fields: 'W10=',
  };

  it('turns an API row into the shape the pages use, with real arrays', () => {
    const s = mapServiceRow(row);
    expect(s.fullName).toBe('Request Money & Split Bill Service');
    expect(s.actionPriority).toBe('before_gl');
    expect(s.channels).toEqual(['Mobile']);
    expect(Array.isArray(s.actions)).toBe(true);
    expect(() => s.actions!.map((a) => a.code)).not.toThrow();
    expect(s.actions).toEqual([]);
    expect(s.inputFields).toEqual([]);
    expect(s.serviceNote).toBeUndefined();
  });

  it('maps a populated pipeline', () => {
    const s = mapServiceRow({ ...row, actions: b64([{ code: 'gl_posting', name: 'GL', type: 'GL Posting', required: true }]) });
    expect(s.actions).toHaveLength(1);
    expect(s.actions![0]!.code).toBe('gl_posting');
  });

  it('never returns a non-array for actions or input fields, even for odd data', () => {
    const s = mapServiceRow({ ...row, actions: b64({ not: 'an array' }), input_fields: b64('text') });
    expect(s.actions).toEqual([]);
    expect(s.inputFields).toEqual([]);
  });
});