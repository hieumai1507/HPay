---
id: BUG-0014
title: The action pipeline calls any URL stored in the database, including internal and metadata addresses, and follows redirects
status: fixed
severity: high
area: backend/gateway
tags: [ssrf, config, authz]
audit: [SSRF in gateway]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/internal/gateway/guard_test.go ; src/backend/internal/workflow/handlers/action_pipeline_test.go (TestActionPipeline_Blocked*)
related: []
---

# BUG-0014: Outbound gateway SSRF

## Symptom
`services.actions[].entry_point` (JSON in the database) was passed straight to `http.NewRequestWithContext` and `client.Do`. Only a string prefix check (`http`) applied. A service configured with `http://169.254.169.254/...`, `http://127.0.0.1:8080/...`, a private address, or a hostile name that resolves to one, made the server call it; redirects (up to 10) could lead anywhere; the method came from configuration. Reachable by anyone able to write a service's actions (a migration or database access today; exposure grows once the catalog becomes editable).

## Root cause
`gateway.HTTP.Call` trusted its input: no scheme restriction beyond `http`, no host or address validation, the default client followed redirects and honoured proxy environment variables, and the pipeline treated every gateway error as a retryable partner outage.

## Other places with the same flaw
Every other outbound HTTP call was searched: the AI advisors call fixed provider hosts, Cloudinary uses a fixed API host, and nothing else builds a URL from stored data. The action pipeline is the only call site of `gateway.HTTP`.

## Fix
Docs `specs/w3-ssrf-guard/spec.md`. `internal/gateway/guard.go`: https only, no credentials, real host, no control characters; every resolved address must be public and the check is made in the dialer on the address actually used (defeats DNS rebinding); blocked ranges are loopback, RFC 1918, carrier-grade NAT, link-local (including the metadata address), unique-local and multicast IPv6, IPv4-mapped forms, unspecified and broadcast; redirects are not followed and a 3xx is a failed call; only POST and PUT are allowed; proxies from the environment are ignored; optional `OUTBOUND_ALLOWED_HOSTS` allowlist (exact and `*.suffix`). Errors and logs carry the host only. The action pipeline treats `ErrBlockedURL` as terminal for a required action and records `:blocked` for an optional one.

## Regression test
`TestGuard_RejectsBadURLs`, `TestGuard_RejectsInternalLiteralIPs`, `TestIsBlockedIPTable`, `TestGuard_BlocksHostnameResolvingInternally` (5 cases including one public plus one private address), `TestGuard_AllowedHosts`, `TestHTTP_DoesNotFollowRedirects_AndSendsIdempotencyKey`, `TestHTTP_ProductionClientRefusesLoopback`, `TestHTTP_MethodRestriction`, `TestHTTP_CallEnforcesURLRulesBeforeAnyNetworkIO`, `TestActionPipeline_BlockedDestinationIsTerminalNotRetryable`, `TestActionPipeline_BlockedOptionalActionIsSkippedWithMarker`. Mutation check: allowing http, removing the CGNAT range, checking only the first resolved address, following redirects, allowing any method, a wrong wildcard rule, and removing the URL check from `Call` each made a test fail (the last one only after adding `TestHTTP_CallEnforcesURLRulesBeforeAnyNetworkIO`).

## How to recognize this again
- Outbound requests from the server to 10.x, 127.x, 169.254.x or to the cloud metadata service.
- A service row whose `entry_point` is `http://` or an IP literal.
- Action pipeline steps retrying a destination that never succeeds.

## Docs updated
`domain/service-catalog.md`, `architecture/crosscutting-concepts.md`, `reference/configuration.md` (`OUTBOUND_ALLOWED_HOSTS`), `constitution.md` (C-10), audit report.

## Not done
No admin endpoint validates `actions` when it is written (there is none yet; when one is added it must call `Guard.CheckURL`). A partner on a private network needs a separate reviewed exception. DNS-level egress controls belong to the deployment.