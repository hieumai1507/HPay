[← M3 API Support](../m3-api-support/implementation-plan.md) · cloudinary-storage · [solution-architecture →](solution-architecture.md)

# Cloudinary Cloud Storage — Implementation Plan

## 1. Scope & non-goals

> **Key point:** Instead of storing KYC files and Excel reports directly on the server's local file system (the `./data/kyc` directory), the system will support configuration that redirects storage to Cloudinary. All logic is abstracted behind the existing `storage.Store` interface, so the KYC and reconciliation business services need no code changes.

| Ships | Non-goals (out of scope here) |
| --- | --- |
| - Add a `Cloudinary` driver implementing the `storage.Store` interface.<br> - Implement the SHA-1 authentication signature mechanism (Signed Upload) to increase security of the Cloudinary API.<br> - Support switching the resource type (`resource_type`): automatically detect KYC image files as `image` and Excel reconciliation reports as `raw`.<br> - Support environment variables to configure and toggle the driver: `STORAGE_PROVIDER=local\|cloudinary`.<br> - Intermediate file download (Get) through an HTTP client so that the backend's ServeFile works seamlessly without directly exposing the Cloudinary storage path. | - No change to the business behavior of the KYC, Reconciliation or Admin handler services.<br> - Do not store absolute Cloudinary URLs in the database (the database keeps relative keys such as `{request_id}/id_card.jpg` to make it easy to switch storage providers).<br> - No integration of other providers (AWS S3, MinIO) in this round. |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Update configuration: declare the variables `STORAGE_PROVIDER`, `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, `CLOUDINARY_API_SECRET` in `config.go` together with fail-fast validation logic. | internal/platform/config/config.go | S | — |
| T2 | Implement the Cloudinary driver: create a new file `cloudinary.go` containing the SHA-1 signature computation, multipart POST upload (Put) and HTTP GET download (Get). | internal/storage/cloudinary.go [NEW] | M | T1 |
| T3 | Router integration: modify `router.go` to initialize the correct `Store` driver based on the `StorageProvider` configuration. | internal/api/router.go | S | T2 |
| T4 | Write unit tests for the Cloudinary driver using a mock `http.RoundTripper` to verify payload, signature and resource_type without calling the real API. | internal/storage/cloudinary_test.go [NEW] | M | T2 |

## 3. Schema changes

No database changes. The `kyc_requests` and `reconciliation_reports` tables keep their existing structure that stores the relative file key as a `string`.

## 4. API changes

No change to the output API structure (OpenAPI Spec). All user and admin endpoints behave exactly as before.

## 5. Test plan

| Group | Test description |
| --- | --- |
| Unit Test (Mock) | Verify the SHA-1 signature matches the Cloudinary algorithm; verify the multipart body contains all required fields; verify the resource type switches to `raw` when the file is `.xlsx`. |
| E2E Test | Ensure the KYC image submission and reconciliation report export flows still complete 100% when the configuration is switched between local and cloudinary. |

## 6. Rollout & risk

- **Risk of wrong credentials:** Mitigated by a fail-fast check at application startup (if cloudinary is selected but credentials are missing, the backend aborts immediately).

- **Fallback capability:** When Cloudinary needs to be turned off or is under maintenance, simply set `STORAGE_PROVIDER=local` in the `.env` configuration file without redeploying the source code.

## 7. Execution checklist

- [ ] gen-feature-plan · [ ] T1 (config) → T2 (cloudinary) → T3 (router) → T4 (tests) · [ ] fintech-review · [ ] commit `feat: add Cloudinary cloud storage driver`
