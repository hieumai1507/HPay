---
title: Wave 3 slice - outbound URL guard (SSRF) for the action pipeline
type: spec
status: proposed
last_verified: 2026-11-21
audit: [SSRF in gateway]
related:
  - ../../domain/service-catalog.md
  - ../../constitution.md
---

# Wave 3 slice: outbound URL guard (SSRF)

## Problem (verified in code)
`workflow/handlers/action_pipeline.go` takes the URL from `services.actions` (JSON in the database) and passes it to `gateway.HTTP.Call` (`gateway.go`), which calls `http.NewRequestWithContext(req.URL)` and `client.Do` directly. The only check is that the string starts with `http`. There is no scheme restriction beyond that, no host allowlist, no block on private, loopback, link-local or cloud-metadata addresses, and the default client follows up to 10 redirects. The method also comes from configuration. Anyone who can write a service's actions (today only a migration or direct database access; no admin endpoint writes them yet, so exposure is limited but real once the catalog becomes editable) can make the server call internal services or `169.254.169.254`.

## Goals
An outbound call can only reach a public HTTPS host, even if the configuration is wrong or hostile, even through DNS tricks and redirects. Constitution C-10.

## Decisions
1. **Scheme:** `https` only. `http` is refused. (A local dev partner uses the Mock gateway; the real gateway is production only, ADR-0012.)
2. **Address check at connect time:** the dialer resolves the host and refuses to connect when ANY resolved address is loopback, private (RFC 1918), link-local (including `169.254.0.0/16`, the metadata range), unique-local IPv6, multicast, unspecified, or `100.64.0.0/10` (carrier-grade NAT). Checking at dial time, on the address actually used, defeats DNS rebinding (a name that resolved to a public address at validation time and to an internal one at connect time).
3. **Literal IPs** in the URL are checked with the same rule.
4. **Redirects:** not followed (`CheckRedirect` returns `http.ErrUseLastResponse`); a 3xx is treated as a failed call.
5. **Method:** only `POST` and `PUT` are allowed; anything else (including empty, which defaults to POST) is normalized or refused.
6. **Optional host allowlist** from `OUTBOUND_ALLOWED_HOSTS` (comma separated exact host names or `*.example.com` suffixes). When set, a host outside it is refused. When unset, any public host passes (an allowlist is the stronger control and is recommended for production; the choice is the operator's).
7. **Credentials in the URL** (`user:pass@host`) are refused. Header and payload handling are unchanged.
8. Response body limit (64 KiB) and the 10 second timeout stay.
9. Validation errors are terminal (retrying cannot fix them), logged with the action code and host, never the full URL.

## Acceptance criteria
- **AC-1** `http://` and any other scheme is refused before any network I/O.
- **AC-2** Literal loopback, private, link-local, metadata, CGNAT and IPv6 internal addresses are refused.
- **AC-3** A hostname that resolves to a private or loopback address is refused at connect time (tested with an injected resolver).
- **AC-4** A hostname that resolves to one public and one private address is refused (every address must pass).
- **AC-5** A redirect (301/302/307/308) is not followed and is reported as a failed call; the redirect target is never contacted.
- **AC-6** With `OUTBOUND_ALLOWED_HOSTS` set, a public host outside the list is refused; an exact and a wildcard match pass.
- **AC-7** URLs with embedded credentials, empty hosts or control characters are refused.
- **AC-8** A valid public HTTPS call still works and still sends the idempotency header (tested against a local TLS test server through a test-only seam that permits the loopback address, which production code cannot enable).
- **AC-9** The action pipeline step treats a validation failure as terminal, not retryable.
- **AC-10** Full suite passes.

## Invariants touched
C-10 (outbound URLs validated). Architecture impact: `amend` (`architecture/crosscutting-concepts.md`, `domain/service-catalog.md`). ADR: none needed beyond this spec (it implements an existing principle without choosing between architectures).

## Constitution check
C-10 enforced; C-11 untouched (URLs are not secrets, but only the host is logged).

## Open questions
1. Allowlist default: **unset (any public host)** so existing production configuration keeps working; recommend setting it.
2. A partner that legitimately lives on a private network (VPN) cannot be reached by the gateway; that needs an explicit, separately reviewed exception and is out of scope.