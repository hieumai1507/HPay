---
title: Wave 3 slice - KYC image upload validation
type: spec
status: proposed
last_verified: 2026-11-21
audit: [BE-H9 upload part]
related:
  - ../../domain/kyc.md
  - ../../constitution.md
---

# Wave 3 slice: KYC image upload validation (BE-H9, upload part)

## Problem (verified in code)
- `kyc.ValidateImage` accepts any buffer whose first 2 bytes are `FF D8` (JPEG) or first 4 are `89 50 4E 47` (PNG): a polyglot or truncated or hostile file passes. The file is never decoded.
- Both images are always stored under the key `.../id_card.jpg` and `.../selfie.jpg`, even when the bytes are PNG, so the stored name lies about the content.
- The handler reads with `io.LimitReader(f, 6<<20)` after Gin has already parsed the multipart body (default 32 MiB held in memory or on disk): an authenticated user can send a very large request or many parts before the cap applies.
- `ServeFile` (`/v1/files/*key`) returns `http.DetectContentType(data)` with no `X-Content-Type-Options: nosniff` and no `Content-Disposition`, so a non-image that slipped through could be rendered by a browser.
- No decompression-bomb limit: a tiny PNG can declare enormous dimensions that exhaust memory when a scorer decodes it.

## Goals
Only real, bounded JPEG or PNG images are stored, under a name that matches their content, and they are served so a browser cannot execute or render them as something else.

## Non-goals
Private/authenticated storage mode and a separate signing key for file links (needs a Cloudinary configuration change and key management: recorded as follow-ups), cleanup of orphaned files, retrying transient storage errors in the KYC scorer (workflow concern).

## Decisions
1. **Decode, do not sniff.** `image.DecodeConfig` must succeed for `image/jpeg` or `image/png`, and the format reported by the decoder decides the extension. A JPEG header followed by garbage that the decoder rejects is refused. (A full `image.Decode` is not needed: `DecodeConfig` validates headers and dimensions cheaply and a truncated body is caught by the optional strict check below.)
2. **Dimension limits:** width and height each between 200 and 8000 pixels and width x height <= 40 megapixels, so a decompression bomb is refused before any pixel is decoded.
3. **Full decode check for size-safe images:** images up to 12 megapixels are fully decoded once to prove the file is not truncated or corrupt; larger ones rely on `DecodeConfig` plus the pixel cap. (Bounded memory: 12 MP x 4 bytes is about 48 MB worst case.)
4. **Byte cap:** 5 MiB per image (unchanged) enforced on the stream, and the whole request body is capped with `http.MaxBytesReader` at 12 MiB (two images plus form overhead) BEFORE multipart parsing; `MaxMultipartMemory` is lowered to 8 MiB.
5. **Names match content:** keys use `.jpg` for JPEG and `.png` for PNG. The recorded key in `kyc_requests` is the real key.
6. **Metadata:** EXIF and other metadata are not rewritten in this slice (re-encoding changes evidence); they are listed as a privacy follow-up.
7. **Serving:** `ServeFile` sets `X-Content-Type-Options: nosniff`, `Content-Disposition: inline; filename="<key basename>"` only for `image/jpeg` and `image/png`, `attachment` for everything else, and `Cache-Control: private, no-store`. The content type comes from the file extension of the key (`.jpg`, `.png`, `.xlsx`), not from sniffing.
8. **Audit:** every admin or signed-link fetch of a KYC image writes an audit row only when the caller is an identified admin; the signed-link route has no identity, so this slice records the access in the access log fields that exist (`request_id`, path without query) and lists identity-bound access as a follow-up (BE-H9 remainder).

## Acceptance criteria
- **AC-1** A real JPEG and a real PNG within limits are accepted and stored with matching extensions.
- **AC-2** A file with a JPEG or PNG signature followed by garbage is refused.
- **AC-3** A GIF, WebP, BMP, SVG, PDF, HTML, ZIP and an executable are refused, even when renamed or sent with an image content type.
- **AC-4** An image smaller than 200 px or larger than 8000 px on a side, or above 40 megapixels, is refused before decoding pixels.
- **AC-5** A truncated image (a valid header, cut body) within the full-decode size is refused.
- **AC-6** A request body above 12 MiB is refused before the images are read into memory (413).
- **AC-7** A PNG is never stored under a `.jpg` key and vice versa.
- **AC-8** `ServeFile` sends `nosniff`, a content type derived from the key extension, `Content-Disposition`, and `Cache-Control: private, no-store`; an unknown extension is served as `application/octet-stream` with `attachment`.
- **AC-9** The existing KYC flow tests still pass with real images (fixtures updated from fake byte strings to generated images).
- **AC-10** Full suite passes.

## Invariants touched
C-11 (PII handled carefully), C-08 partially (file access identity remains a follow-up). Architecture impact: `none` beyond `domain/kyc.md` and `domain/identity-and-auth.md` rule updates.

## Constitution check
C-11 improved (hostile files cannot be stored or rendered). No deviation. The identity-bound file access gap is stated, not hidden.

## Open questions
1. Minimum 200 px may reject low-resolution phone cameras on old devices; **assumption: 200 px on the short side is acceptable for an ID document photo**. Change the constant if product disagrees.
2. EXIF stripping (location data in selfies): privacy decision, not done here.