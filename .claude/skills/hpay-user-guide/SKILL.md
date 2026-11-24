---
name: hpay-user-guide
description: Write, update and verify the illustrated HPay user guides for non-technical staff (admin console now, mobile app later). Use whenever a change touches what a person sees or does in the admin or mobile UI (a page, a label, a button, a flow, a role or permission, an error message), when asked to write or refresh a user guide, or at hpay-feature stage 2 and stage 9 for any UI change. Covers the writing rules, retaking screenshots with numbered callouts, the "what changed" note, the self-contained HTML copy and the automated checks. Not for developer how-tos in docs/guides (those are plain engineering guides).
---

# hpay-user-guide

The guide is how back-office staff learn the product. If the UI changes and the guide does not, staff follow steps that no longer exist. So **a UI change is not done until the guide matches it** (hpay-feature stage 9 checks this).

Files (admin guide; a mobile guide follows the same layout under `docs/guides/mobile/`):
- `docs/guides/admin-user-guide.md`: source of truth. English only.
- `docs/guides/admin-user-guide.html`: self-contained copy (images embedded, print CSS, no network). Generated, never edited by hand.
- `docs/guides/admin/img/NN-name.png`: screenshots with numbered red callouts.
- `docs/guides/admin/WRITING-RULES.md`: the 15 writing rules. Read it before editing.
- `docs/guides/admin/check-guide.mjs`: the checker (links, orphans, HTML, forbidden words).
- `docs/guides/admin/OPEN-ISSUES.md`: product problems found while writing (a page looked broken, a label confused you). These are bugs for `hpay-bugfix` or `hpay-feature`, not things to hide with wording.

## When to run it
| Change | What to do |
|---|---|
| New page, tab, dialog or button | New or rewritten chapter section, new screenshot(s), glossary entries for new terms |
| Renamed label, moved button, new step in a flow | Edit every step and bold label that names it, retake the screenshots that show it |
| Role or permission change | Update the roles table (section 4) from the code, and every "you may not see this" sentence |
| New or changed error message the staff will see | Update the "If something looks wrong" box and the FAQ |
| Page removed or gated off | Remove it, or turn it into a **Not available yet** box that says what the screen says |
| Pure backend or invisible change | Nothing; write "guide: not affected" in the spec |

## Procedure
1. **Find what changed.** From the diff list the screens, labels, buttons, roles and messages a user can see. Grep `src/admin/src` (or `src/mobile/app`) for each label so the bold names in the guide are exact.
2. **Read the rules** in `docs/guides/admin/WRITING-RULES.md`. The short version: short sentences (under 20 words), active voice and "you", one action per numbered step starting with a verb, exact on-screen label in **bold**, an italic *You should see:* line after each step that changes the screen, a **Careful** box before anything that moves money or cannot be undone, a **Not available yet** box for anything the screen lacks, no developer words in the body (API, endpoint, JSON, idempotent, workflow engine), every term explained on first use, at most two printed pages per chapter, alt text and "Figure N:" captions.
3. **Edit the chapter.** Keep the chapter shape: What this page is for / When you need it / numbered tasks / What the columns and statuses mean (table) / Careful / If something looks wrong. Describe only what exists today; never describe a feature the product lacks.
4. **Retake the screenshots** that show a changed screen (see below). A changed spacing or label changes the picture too, so retake instead of patching.
5. **Add a line** to "What changed in this version" at the top (date, chapter, one sentence) and bump the version line.
6. **Regenerate the HTML** (see below) and run the checker. Paste its output in your report.
7. **Report product problems** you noticed to `OPEN-ISSUES.md` and to the user.

## Screenshots (Playwright)
Playwright is a devDependency of `src/admin` (`npx playwright install chromium` once). Write the capture script **outside the repo** (under `%TEMP%`), log in through the real login form, use 1440x900, wait for network idle, and inject numbered red badges and a 3 px outline over the exact element each step names before taking the picture (see `scripts/capture-example.mjs` for the pattern). Rules:
- The app must run on demo data (`cmd/seed`); never capture real customer data.
- Look at every picture after capturing it. If the callout is on the wrong element or the page looks broken, stop and record the problem instead of shipping the picture.
- PNG under 250 KB each (clip to the relevant area when the full page is noisy).
- Capture write dialogs BEFORE the confirm button and cancel; never perform a write only to illustrate it unless it is harmless and you restore it.
- Same file name for the same figure, so links keep working; delete screenshots no longer referenced.
- Take the screenshot for each role when the page differs by role.

## HTML copy
Convert the markdown to one HTML file with images embedded as base64, print-friendly CSS, a table of contents and no external requests. `scripts/build-html.mjs` does this with no dependencies (a tiny converter that covers the subset of Markdown the guide uses: headings, paragraphs, lists, tables, blockquote callouts, images, bold, italic, code). Run `node docs/guides/admin/check-guide.mjs` afterwards.

## Checker (`check-guide.mjs`)
Fails when: an image link points to a missing file; an image file is not referenced; an embedded image is missing from the HTML; a Vietnamese letter, "sample data", "TanStack" or "lorem" appears; a bold label that looks like a button is not found in the admin source (warning list, not a failure, because some labels are data); a chapter has more than N numbered steps in one list (default 9). The wording "API coming" is allowed only inside a **Not available yet** callout that quotes what the screen says.

## Guardrails
- English only, no Vietnamese anywhere (the product is English-only).
- No credentials in the guide except the documented demo logins, and mark them "practice system only".
- The guide never contradicts `docs/domain/*`; when they disagree, the code is the fact (see AUDIT-REPORT), fix the domain doc in the same change.
- File encoding on Windows PowerShell 5.1: do not use `Set-Content -Encoding utf8` (adds a BOM) or `Get-Content` without `-Encoding UTF8`; write with the editor tools or `[IO.File]::WriteAllText(path, text, (New-Object System.Text.UTF8Encoding($false)))`.
- Do not add new tooling or dependencies for the guide; Node built-ins plus Playwright (already present) only.

## Definition of done
Every changed screen is described as it is now; screenshots retaken and viewed; "What changed" updated; HTML regenerated; `check-guide.mjs` passes; product problems recorded; the spec or PR says "guide: updated" or "guide: not affected" with the reason.