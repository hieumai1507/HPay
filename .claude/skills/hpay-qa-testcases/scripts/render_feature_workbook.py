# -*- coding: utf-8 -*-
"""
Render ONE feature's QA workbook from a data file. This script holds no test cases.

The cases come from the feature itself: the skill designs them (hpay-feature stage 4) and
writes them to docs/specs/<slug>/test-cases.json next to the spec. This script only turns that
file into docs/qa/test-cases/<slug>-testcase.xlsx using the same sheet format as the five
catalog workbooks (same columns, validations, Read Me sheet). It never touches the catalog
workbooks, so recorded results there cannot be erased.

Usage (repo root):
  python -X utf8 .claude/skills/hpay-qa-testcases/scripts/render_feature_workbook.py <slug> [--data FILE] [--out FILE]
  (a feature reads docs/specs/<slug>/test-cases.json; a bug passes --data docs/bugs/BUG-NNNN-testcases.json)

Data file shape (docs/specs/<slug>/test-cases.json):
  {"slug": "...", "title": "...", "spec": "...", "adr": "...",
   "cases": [{"id", "ac", "feature", "role", "type", "priority", "precondition", "steps", "data",
              "expected", "edge", "executor", "method", "status", "actual", "evidence",
              "executed_by", "last_run", "notes"}, ...]}
`status` and the result fields are the recorded outcome; update them in the JSON after a run
and render again. Re-rendering is safe: the JSON is the single source of truth.
"""
import argparse
import importlib.util
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def _load_format():
    spec = importlib.util.spec_from_file_location("gen_format", os.path.join(HERE, "generate_test_cases.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


REQUIRED = ["id", "feature", "role", "type", "priority", "precondition", "steps", "data", "expected", "executor"]


def to_row(case):
    missing = [k for k in REQUIRED if not str(case.get(k, "")).strip()]
    if missing:
        raise SystemExit(f"case {case.get('id', '?')}: missing {missing}")
    return [
        case["id"], case["feature"], case["role"], case["type"], case["priority"],
        case["precondition"], case["steps"], case["data"], case["expected"],
        case.get("edge", ""), case["executor"], case.get("method", ""),
        case.get("status", "Not Run"), case.get("actual", ""), case.get("evidence", ""),
        case.get("executed_by", ""), case.get("last_run", ""), case.get("notes", ""),
    ]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("slug")
    ap.add_argument("--data", default=None, help="data file; default docs/specs/<slug>/test-cases.json")
    ap.add_argument("--out", default=None)
    args = ap.parse_args()
    fmt = _load_format()
    src = args.data or os.path.join(fmt.REPO_ROOT, "docs", "specs", args.slug, "test-cases.json")
    if args.data and not os.path.isabs(src):
        src = os.path.join(fmt.REPO_ROOT, src)
    if not os.path.exists(src):
        raise SystemExit(f"no data file: {src}")
    with open(src, encoding="utf-8") as fh:
        doc = json.load(fh)
    rows = [to_row(c) for c in doc["cases"]]
    ids = [r[0] for r in rows]
    if len(ids) != len(set(ids)):
        raise SystemExit("duplicate test ids in " + src)
    for r in rows:
        if r[3] not in fmt.TEST_TYPES or r[4] not in fmt.PRIORITIES or r[10] not in fmt.EXECUTORS or r[12] not in fmt.STATUSES:
            raise SystemExit(f"{r[0]}: type, priority, executor or status is not an allowed value")
    out = args.out or os.path.join(fmt.OUT_DIR, f"{doc['slug']}-testcase.xlsx")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    fmt.build_workbook(f"{doc['slug']}-testcase.xlsx", rows, out)
    print("source:", os.path.relpath(src, fmt.REPO_ROOT))


if __name__ == "__main__":
    sys.exit(main())