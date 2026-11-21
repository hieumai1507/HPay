[solution-architecture →](solution-architecture.md) · m5-business-wallet

# M5 Business Wallet — Implementation Plan

## 1. Scope

Build the Business Wallet UI and the Maker/Checker approval flow on mobile. Every transaction is controlled through secure APIs protected by the Checker's OTP signature.

## 2. Tasks

| # | Task | Main files | Size | Depends |
|---|---|---|---|---|
| T0 | Run `npm run gen:api` to refresh the types after the Backend adds the `GET /v1/org` endpoint | `src/api/` | S | backend GET /v1/org |
| T1 | Set up the `useOrgStore` store that manages the currently selected organization state | `src/stores/org.ts` | S | — |
| T2 | Add the business wallet picker section and the create-organization form to the Account screen | `app/(main)/account.tsx` | M | T1 |
| T3 | Implement the Home UI in business mode according to the member's role | `app/(main)/home.tsx` | M | T1 |
| T4 | Maker screen for creating a disbursement order | `app/(main)/org/disburse.tsx` | M | T0 |
| T5 | Approvals queue screen for the Checker | `app/(main)/org/approvals.tsx` | M | T0 |
| T6 | Approval detail screen + OTP Challenge + Decide | `app/(main)/org/approval-detail.tsx` | L | T5 |
| T7 | Admin UI for managing Members and Policies | `app/(main)/org/admin.tsx` | M | T0 |
| T8 | Write unit tests for the store logic and the helper logic | `src/stores/__tests__/org.test.ts` | S | T1 |

## 3. Routes Mapping (expo-router)

Add the new Stack screens to `app/(main)/_layout.tsx` with the flag that hides the tab bar:

```
<Tabs.Screen name="org/disburse" options={{ href: null, tabBarStyle: { display: 'none' } }} />
<Tabs.Screen name="org/approvals" options={{ href: null, tabBarStyle: { display: 'none' } }} />
<Tabs.Screen name="org/approval-detail" options={{ href: null, tabBarStyle: { display: 'none' } }} />
<Tabs.Screen name="org/admin" options={{ href: null, tabBarStyle: { display: 'none' } }} />

```

## 4. Verification

Run the quality checks before submitting for review:

```
npm run typecheck
npm run lint
npm test

```
