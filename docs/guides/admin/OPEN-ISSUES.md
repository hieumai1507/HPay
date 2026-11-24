# Open issues found while updating the admin guide (2026-11-24, second revision)

Product problems seen in the running app (admin UI on :5173, API on :8080, demo data). None is hidden by wording in the guide. Each item below was checked again in the app on this date.

Fixed since the first revision and verified gone: policy list loading (old item 1), empty KYC photo boxes (2), blank Transaction and Amount on workflow detail and raw step data (3), "undefined" limits on offers (part of 10), technical wording in the Fraud Monitor title and the AI Settings title and "(F11)" (part of 9), Service Catalog showing "(0 steps)" for P2P transfer (6, partly: see item 5 below).

## Still open

1. **Fraud Monitor: Amount and User columns show a dash** for the demo flag, and the confirm window says "for user —". The data is missing for that flag, the page does not look it up.
2. **Audit Log is empty** in the demo although admin actions exist (seed and tests). The guide cannot show a row.
3. **Dashboard: four daily cards and the "Volume by hour" chart are empty** ("(no analytics API yet)", "no data yet") until an analytics endpoint exists.
4. **Browser pop-ups instead of in-page messages** on Fraud Monitor (confirm, mark safe), Service Catalog (confirm status change), Reconciliation (run result, export), Workflow detail (missing reason, errors) and AI Settings (saved, errors). The Resolve button on Reconciliation only shows a pop-up. The Fraud resolve window also uses a pop-up for a short reason.
5. **Service Catalog: four of five services show "(0 steps)"** and "Trans Validations (0)". The Overview tab has boxes that look editable but nothing can be saved.
6. **Developer wording still on screen:** "Service catalog (Low-Code Configurator)", the amber banner "no POST /v1/admin/services", the plan banner "only GET /v1/admin/offers ...", the AI Settings subtitle "Dynamic LLM Provider & Secret Management", the Fraud confirm window ("saga", "risk_officer role"), the AI model name "mock", and the tooltips on the grey buttons. The guide describes these without repeating the technical parts.
7. **Offer detail keeps the limit boxes as raw numbers** (for example 100000000) with the formatted amount underneath. Save Plan settings looks active (teal, not greyed) but is disabled.
8. *(fixed 2026-11-24)* "+ New plan" is now greyed out with an explanation instead of opening a page that never loads.
9. *(fixed 2026-11-24)* Workflow detail shows Retry and Compensate only for a payment that failed.
10. *(fixed 2026-11-24)* The yellow note on Organizations now has its missing spaces.
11. **Ops can open Organizations & Policy** but sees the note "(orgs.edit)", a technical permission name.
12. The menu entry is "Workflow" while the page title and the top bar say "Workflow Monitor". (The status now reads "Waiting approval" in the list.)
