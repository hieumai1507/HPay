[← solution-architecture](solution-architecture.md) · a5-offer-service-factory

# A5 Offer Factory & Service Catalog — Implementation Plan

## 1. Scope & Deliverables

Add an administration module for Product Offers and financial Services to `hpay-admin`:

- **Service Catalog Manager** screen (`/services`): manage the service catalog, toggle Maintenance Mode on/off.
- **Offer Factory List** screen (`/offers`): list of product offers by user tier.
- **Offer Detail Builder** screen (`/offers/:id`): a 4-step configurator (Basic info, Pocket Limits, Authorized Services, Service Components - Limit Matrix & Authentication Matrix).

## 2. Task Breakdown

| # | Task | Files | Size | Depends |
|---|---|---|---|---|
| T1 | Service Catalog hooks: `useServices()`, `useUpdateServiceStatus()`, `useCreateService()` | `src/features/services/hooks.ts` | S | — |
| T2 | Service Catalog Page screen: services table, Maintenance Mode toggle switch, modal to add a new service | `src/pages/ServiceCatalogPage.tsx` | M | T1 |
| T3 | Offer Factory hooks: `useOffers()`, `useOfferDetail(id)`, `useSaveOffer()` | `src/features/offers/hooks.ts` | S | — |
| T4 | Offer Factory List Page screen: grid of Offer cards (Customer vs Business), level info, role, active status | `src/pages/OfferFactoryPage.tsx` | M | T3 |
| T5 | Offer Detail Builder Page screen: 4-step wizard (Basic Info, Pocket Limits, Authorized Services, Service Component Limits & Auth Matrix Editor) | `src/pages/OfferDetailPage.tsx` | L | T3,T4 |
| T6 | Update `Sidebar.tsx` (add the Services & Offers menu) and `router.tsx` (routes `/services`, `/offers`, `/offers/:id`) | `src/ui/Sidebar.tsx`, `src/router.tsx` | S | T2,T5 |
| T7 | Quality check (typecheck, lint, build) & commit push code | All project files | S | T6 |

## 3. Verification & Definition of Done

```
npm run typecheck    # 0 TypeScript errors
npm run lint         # 0 ESLint errors
npm run build        # Vite build succeeds
git push origin main
```

hpay-admin · A5 Implementation Plan v1 · 2026
