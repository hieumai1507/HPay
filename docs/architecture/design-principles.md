---
title: Design principles (SOLID, patterns, smells)
type: architecture
status: unverified
last_verified: 2026-11-26
code: [src/backend/internal, src/admin/src, src/mobile/src]
---

# Design principles

How we write code that stays easy to change. This page is **guidance for implementers and reviewers**. It never overrides `../constitution.md` (C-01..C-15): where they disagree, the constitution wins. Distilled from the SOLID/clean-code skill collections reviewed in `D:\hpay-skills-research` (ramziddin/solid-skills, addyosmani agent-skills code-review and code-simplification), keeping only what fits a Go + React + Expo fintech codebase.

## 0. Meta-rules (read first)

1. **Patterns are found, not planned.** Use a pattern only when the problem is already in front of you (a second real variant, a real test seam). No abstraction "just in case" (YAGNI).
2. **Rule of three.** Tolerate two copies; extract on the third. A small duplication beats the wrong abstraction.
3. **Prefer deleting an abstraction to polishing it.** A refactor must reduce the number of concepts a reader holds, not just move them.
4. **Follow the neighbours.** Match the style of the surrounding package before importing outside taste. New patterns need a reason in the plan.
5. **Refactor only on green tests**, in a separate commit from the behavior change, with no behavior change.

## 1. SOLID, translated to HPay

| Principle | Question | In this repo |
|---|---|---|
| **S** Single responsibility | One reason to change? | A package = one bounded context (`fee`, `ledger`, `kyc`). A handler parses/validates HTTP and calls a domain function; it holds no business rules or SQL. A step handler does one workflow step. |
| **O** Open/closed | Can I add a variant without editing working code? | New workflow step = new `StepHandler` registered in `workflow/registry.go`, not another `switch` in the engine. New LLM or storage provider = new implementation of `ai.Advisor` / `storage.Store`. Workflows are data (`workflow_definitions`), so a flow change is a migration, not an engine edit. |
| **L** Liskov | Can an implementation replace another safely? | Every implementation of `gateway.Gateway`, `storage.Store`, `auth.Sender`, `ai.Advisor` honours the same contract: same errors, same idempotency, same context cancellation. The mock must not be more lenient than the real adapter. |
| **I** Interface segregation | Does the caller depend on methods it never uses? | Go: define small interfaces **in the consumer package**, 1 to 3 methods (see `PauseHook`, `Authenticator`). Do not export a wide interface next to its single implementation. |
| **D** Dependency inversion | Does high-level policy depend on a concrete detail? | Domain packages depend on ports (interfaces), `cmd/server/main.go` wires the concrete adapters. Nothing imports `api`. Direction stays `api -> domain -> ledger/workflow -> store/gen`. The `approval` -> `workflow` link through `PauseHook` is the model to copy for any would-be import cycle. |

Frontends: SRP means a component renders, a hook fetches (TanStack Query), a pure function computes (`mobile/src/lib/money.ts`). DIP means screens call the generated API client through hooks, never `fetch` inline.

## 2. Design patterns that fit (and where HPay already uses them)

| Pattern | Use when | HPay example / candidate | Do not |
|---|---|---|---|
| **Port and adapter (Adapter)** | An external system or provider varies | `gateway`, `storage`, `auth.Sender` (Twilio), `ai` clients | Wrap a library that will never be swapped and has no test need |
| **Strategy** | Several interchangeable algorithms chosen at runtime | fee calculation per policy, approval tier rules, auth method per offer | Introduce for a single algorithm |
| **Registry / Factory function** | Look up behavior by key from data | `workflow/registry.go` step handlers by name | Build factory classes in Go; a `func New...` or a map is enough |
| **Template method via composition** | Same skeleton, different steps | engine run loop with injected step handlers | Inheritance-style abstract bases (Go has none, TS: prefer composition) |
| **Decorator / middleware** | Cross-cutting behavior around a call | Gin middleware (auth, rate limit, idempotency); logging or retry wrappers around a `Gateway` | Hide business rules in middleware |
| **State machine** | Lifecycle with guarded transitions | `workflow` engine, approval, reversal (C-05: only the owning transition function changes status) | Raw `UPDATE ... SET status` outside it |
| **Outbox** | Reliable event after a state change | `outbox` (C-06) | Publish to Redis directly from a handler |
| **Command / use-case function** | One business operation with input and result | `transfer.ExecuteEngineRequest`, reversal execute | A `Manager` or `Helper` that groups unrelated operations |
| **Value object** | A primitive carries rules | amounts in integer minor units (C-01), phone masking, ids | Wrapping every string; wrap only where there are invariants (money, currency, phone, PIN) |
| **Repository** | Hide persistence | already provided by sqlc `store/gen`; do not add a second repository layer on top | Hand-written ORM-style layers over sqlc |
| **Builder (tests)** | Many optional fields | `test/testkit` fixtures | Builders in production code when a struct literal works |
| **Singleton** | Almost never | pass dependencies explicitly from `main.go` | Package-level mutable globals |

## 3. Go specifics (backend)

- Accept interfaces, return structs. Interfaces belong to the consumer.
- Constructors `NewX(deps...)` inject dependencies; no hidden globals or `init()` side effects.
- `context.Context` first parameter on anything that does I/O; honour cancellation.
- Errors: wrap with `%w`, map to the API contract only in `api/errmap.go`. No string matching on errors, no swallowed errors (code review flags ignored `err`).
- Parameter lists over about 4: introduce an options/input struct.
- Early return and guard clauses; keep nesting shallow. Functions short enough to name what they do in one sentence.
- Transactions: begin, defer rollback, one place commits. Side effects (HTTP, Redis) never inside the DB transaction (C-06).
- Table-driven tests, name states a concrete example ("transfer of 100 with balance 99 returns ErrInsufficientFunds").

## 4. React / Expo specifics

- Split by responsibility: page or screen (layout and wiring) -> hook (data, TanStack Query) -> pure helper (formatting, money). Cross-reference `hpay-admin-recipes` and `hpay-mobile-recipes`.
- Extract a component or hook on the third repetition, not the first.
- Props are narrow and typed; no passing whole API objects when two fields are used.
- State: server state in TanStack Query, UI state local, shared client state in zustand only when really shared.
- Never compute money with `Number()`/floats (C-01).

## 5. Code smells to flag in review

| Smell | Typical HPay form | Remedy |
|---|---|---|
| Long function / deep nesting | an HTTP handler that validates, queries, calculates fees and writes | guard clauses, extract the business step into the domain package |
| Switch or if-chain on a type or status | `if kind == "p2p" ... else if kind == "org"` growing | strategy or registry lookup, or put the variant into data |
| Shotgun surgery | one rule edited in `api`, `transfer`, `org` and the admin app | move the rule next to its owner (domain package) and call it |
| Feature envy | a package reading another context's tables | call the owning package; no cross-context SQL (check `domain/*.md`) |
| Primitive obsession | `amount int64, currency string` passed loosely | a small value type where invariants exist |
| Speculative generality | an interface with one implementation and no test fake | delete the interface until a second use or a test needs it |
| Bolted-on conditional | a new flag branch in an unrelated flow | give it its own function, step, or policy |
| God package or helper | `utils`, `common`, `manager` | split by context, name by what it does |
| Hidden global state | package-level maps, singletons | inject from `main.go` |

## 6. How this plugs into the workflow

- **hpay-feature stage 3 (plan / design gate)**: add a short "Design check" under the constitution check: which responsibilities are added to which package; any new interface (who is the consumer, which second implementation or test fake justifies it); any new pattern and why the existing ones do not suffice. A new pattern with no concrete second use is a plan defect.
- **Implementer**: during the REFACTOR step of red-green-refactor, apply sections 1 to 5 to the code just written, inside the owned file set. Do not widen the diff.
- **Reviewers** (`code-reviewer`, `hpay-review`): check section 5 smells and the meta-rules; every finding still needs a concrete scenario (what gets harder to change, or what breaks).
- Violations that already exist are not fixed opportunistically inside a feature; log them (audit report) or fix them in a dedicated refactor task.

## 7. Design check (copy into plan.md)

| Question | Answer |
|---|---|
| Which package owns each new responsibility (SRP)? | |
| New interface? Consumer package, 1 to 3 methods, second implementation or test fake that justifies it | |
| Does a new variant require editing a working `switch`/engine (OCP)? Registry or data instead? | |
| Dependency direction still `api -> domain -> ledger/workflow -> store/gen`? | |
| New pattern introduced? Problem it solves today (not tomorrow) | |
| Duplication: is this the third copy? | |
