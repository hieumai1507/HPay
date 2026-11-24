# Open issues found while updating the admin guide (2026-11-24)

Product problems seen in the running app. None is hidden by wording in the guide.

1. **Organizations & Policy showed "Could not load the approval policies: Request failed with HTTP 404".** The API container on port 8080 was started before the route `GET /v1/admin/orgs/:id/policies` existed, so it answers 404. The screenshots 19 and 29 were taken with the browser route for that one call answered by a second API process built from the current code (same database, read only). Restart the main API to fix.
2. **KYC detail: both photo boxes are empty** (broken image icon with the alt text "CCCD" and "Selfie"). The demo has no photo files. The guide says so in a Not available yet box.
3. **Workflow detail: Transaction and Amount show "—" and Workflow type shows "workflow"** for the failed payment, although the list row shows the code and 200,000 ₫. The timeline shows raw base64 text ("out: e30=") and the failed step is listed three times.
4. **Fraud Monitor: Amount and User columns show "—"** for flag #c751, and the confirm window says "for user —".
5. **Audit Log is empty** in the demo, although admin actions exist (seed and e2e). No row can be shown in the guide.
6. **Service Catalog: four of five services show "(0 steps)"**, and Trans Validations shows (0) for the one captured. The Overview tab has editable-looking boxes that save nothing.
7. **Dashboard: "Volume by hour" shows a lone dash with "no data yet"** and four cards stay empty until an analytics endpoint exists.
8. **Reconciliation and Service Catalog use browser pop-ups** (`alert` / `confirm`) instead of in-page messages. The Resolve button only shows a pop-up.
9. **Developer wording still on screen:** "Service catalog (Low-Code Configurator)", the amber banner "no POST /v1/admin/services", the plan banner "(only GET /v1/admin/offers ...)", the workflow box "on_failure: compensate", and the AI Settings subtitle "Dynamic LLM Provider & Secret Management". The guide describes these without repeating the technical parts.
10. **Offer detail** keeps the limit boxes as raw numbers (for example 62500000) with the formatted amount underneath.
