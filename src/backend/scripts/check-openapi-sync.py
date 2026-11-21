#!/usr/bin/env python3
"""CI gate (user rule): every route in internal/api/router.go must exist in
api/openapi.yaml and vice versa. Drift fails the build.
Zero-dependency: uses PyYAML when available, falls back to a line parser."""
import re
import sys

group_prefix = {"r": "", "v1": "/v1", "ag": "/v1/auth", "me": "/v1", "org": "/v1/org", "og": "/v1/org",
                "adm": "/v1/admin", "adg": "/v1/admin", "fg": "/v1/files"}
METHODS = ("get", "post", "put", "patch", "delete")


def routes_from_router():
    src = open("internal/api/router.go", encoding="utf-8").read()
    out = set()
    for m in re.finditer(r'\b(\w+)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]*)"', src):
        var, method, path = m.groups()
        prefix = group_prefix.get(var)
        if prefix is None:
            continue
        full = re.sub(r"[:*](\w+)", r"{\1}", prefix + path) or "/"
        out.add((method.lower(), full))
    return out


def ops_from_spec():
    try:
        import yaml  # type: ignore
        spec = yaml.safe_load(open("api/openapi.yaml", encoding="utf-8"))
        return {(m, p) for p, ops in spec.get("paths", {}).items()
                for m in ops if m in METHODS}
    except ImportError:
        return ops_from_spec_fallback()


def ops_from_spec_fallback():
    """Line-based parse: paths are 2-space keys starting with '/' under
    'paths:'; methods are 4-space keys. Good enough for our stable format."""
    ops, current, in_paths = set(), None, False
    for line in open("api/openapi.yaml", encoding="utf-8"):
        if re.match(r"^paths:\s*$", line):
            in_paths = True
            continue
        if in_paths and re.match(r"^\S", line):  # next top-level key
            break
        if not in_paths:
            continue
        m = re.match(r"^  (/\S*):\s*$", line)
        if m:
            current = m.group(1)
            continue
        m = re.match(r"^    (\w+):\s*$", line)
        if m and current and m.group(1) in METHODS:
            ops.add((m.group(1), current))
    return ops


routes, spec_ops = routes_from_router(), ops_from_spec()
missing_in_spec = sorted(routes - spec_ops)
missing_in_code = sorted(spec_ops - routes)
if missing_in_spec:
    print("ROUTES MISSING IN openapi.yaml:")
    for m, p in missing_in_spec:
        print(f"  {m.upper()} {p}")
if missing_in_code:
    print("SPEC PATHS WITH NO ROUTE (stale spec?):")
    for m, p in missing_in_code:
        print(f"  {m.upper()} {p}")
if missing_in_spec or missing_in_code:
    sys.exit(1)
print(f"openapi sync OK: {len(routes)} operations match")
