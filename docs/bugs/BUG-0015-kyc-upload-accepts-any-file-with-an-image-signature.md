---
title: KYC upload accepts any file that starts with a JPEG or PNG signature and serves files without nosniff
id: BUG-0015
status: fixed
severity: high
area: backend/kyc
tags: [upload, pii, authz]
audit: [BE-H9]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/internal/kyc/image_test.go ; src/backend/internal/api/kyc_upload_test.go ; src/backend/test/integration/kyc/kyc_upload_test.go
related: [BUG-0013]
---

# BUG-0015: KYC upload validation and file serving

## Symptom
Any file whose first 2 bytes were `FF D8` or first 4 were `89 50 4E 47` was accepted and stored, whatever followed (HTML, scripts, junk, truncated images). PNG files were stored under a `.jpg` name. The whole multipart request was parsed (up to 32 MiB held in memory or on disk) before the 6 MiB cap applied. A tiny PNG could declare enormous dimensions. `GET /v1/files/*key` answered with a content type sniffed from the bytes and no `nosniff`, so a non-image that got through could be rendered by a browser.

## Root cause
`kyc.ValidateImage` compared a few leading bytes and never decoded; the keys were built with a hard-coded `.jpg`/`.jpg`; the handler read with `io.LimitReader` after Gin had already parsed the form; `ServeFile` used `http.DetectContentType`.

## Other places with the same flaw
`ServeFile` also serves reconciliation `.xlsx` files; it now treats them as attachments. The two scorers still pick a media type with `http.DetectContentType` for the LLM request; harmless once only real jpeg/png are stored, and left as is.

## Fix
`docs/specs/w3-kyc-upload/spec.md`. `kyc.InspectImage` decodes with the standard library: only `jpeg` and `png` by format name, 200 to 8000 pixels per side, at most 40 megapixels (checked from the header before any pixel is decoded), a full decode up to 12 megapixels to catch truncated or corrupt files, 5 MiB cap. The stored key extension follows the content (`KeyFor`). The whole request body is capped at 12 MiB with `http.MaxBytesReader` before multipart parsing (413 `ERR_PAYLOAD_TOO_LARGE`) and `MaxMultipartMemory` is 8 MiB. `ServeFile` derives the type from the key extension, sets `X-Content-Type-Options: nosniff`, `Cache-Control: private, no-store`, a sanitized `Content-Disposition` (inline only for jpeg and png), and serves unknown extensions as `application/octet-stream` attachments.

## Regression test
`TestInspectImage_*` (real jpeg and png, signature plus garbage, other formats including a GIF refused by name, dimension limits, truncated bodies, byte cap), `TestKeysMatchContent`, `TestKYCSubmitBodyIsCappedBeforeParsing`, `TestServeFileHeaders`, `TestServeFileFilenameIsSanitized`, and through the real router `TestKYCUpload_RealImagesAcceptedWithMatchingKeys`, `_PolyglotRefusedAndNothingStored`, `_OversizeBodyRefusedWith413`. The old 1x1 test PNG was replaced with a generated 400x400 image. Mutation check: removing the format allowlist together with the full decode, the minimum size, the pixel cap, the truncation check, or reporting PNG as JPG each made a test fail. Removing the format allowlist alone does not fail a test because the full decode refuses a GIF in the next step (two overlapping layers); `TestInspectImage_AllowlistIsByFormatName` pins the behavior.

## How to recognize this again
- A stored KYC file whose bytes do not decode as an image.
- A `.jpg` key that holds PNG data.
- Memory spikes on `/v1/kyc/submit`.
- A browser rendering a file from `/v1/files/...` as a page instead of downloading or showing an image.

## Docs updated
`domain/kyc.md`, `domain/identity-and-auth.md`, `reference/api.md`, `reference/error-codes.md`, `constitution.md` (C-11), audit report.

## Not done (BE-H9 remainder)
Private or authenticated storage mode and URLs for the image host (Cloudinary uploads are public by URL today), a separate signing key for file links (it is still the JWT secret), identity-bound and audited access to KYC images (the signed link has no identity), EXIF stripping, cleanup of orphaned files, and retry of transient storage errors in the KYC scorer.