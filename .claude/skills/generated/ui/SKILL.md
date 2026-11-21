---
name: ui
description: "Skill for the Ui area of hpay. 7 symbols across 2 files."
---

# Ui

7 symbols | 2 files | Cohesion: 100%

## When to Use

- Working with code in `hpay-mobile/`
- Understanding how sanitizeAmountInput, parseAmountToBigInt, formatAmountWhileTyping work
- Modifying ui-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-mobile/src/lib/money.ts` | sanitizeAmountInput, parseAmountToBigInt, groupThousands, formatAmountWhileTyping, formatAmountDisplay (+1) |
| `hpay-mobile/src/ui/AmountDisplay.tsx` | AmountDisplay |

## Entry Points

Start here when exploring this area:

- **`sanitizeAmountInput`** (Function) — `hpay-mobile/src/lib/money.ts:10`
- **`parseAmountToBigInt`** (Function) — `hpay-mobile/src/lib/money.ts:18`
- **`formatAmountWhileTyping`** (Function) — `hpay-mobile/src/lib/money.ts:38`
- **`formatAmountDisplay`** (Function) — `hpay-mobile/src/lib/money.ts:45`
- **`formatAmountVND`** (Function) — `hpay-mobile/src/lib/money.ts:53`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `sanitizeAmountInput` | Function | `hpay-mobile/src/lib/money.ts` | 10 |
| `parseAmountToBigInt` | Function | `hpay-mobile/src/lib/money.ts` | 18 |
| `formatAmountWhileTyping` | Function | `hpay-mobile/src/lib/money.ts` | 38 |
| `formatAmountDisplay` | Function | `hpay-mobile/src/lib/money.ts` | 45 |
| `formatAmountVND` | Function | `hpay-mobile/src/lib/money.ts` | 53 |
| `AmountDisplay` | Function | `hpay-mobile/src/ui/AmountDisplay.tsx` | 23 |
| `groupThousands` | Function | `hpay-mobile/src/lib/money.ts` | 25 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `AmountDisplay → GroupThousands` | intra_community | 3 |

## How to Explore

1. `context({name: "sanitizeAmountInput"})` — see callers and callees
2. `query({search_query: "ui"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
