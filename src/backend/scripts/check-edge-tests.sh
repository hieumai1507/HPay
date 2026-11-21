#!/usr/bin/env bash
# Fails CI if an edge case ID listed in docs has no matching TestEdge_<ID>_ test.
# IDs are introduced per phase; extend REQUIRED as phases land (master plan §4).
set -euo pipefail
# F10: the full catalogue E01–E20 is the default gate (master plan §4).
ALL_IDS="E01 E02 E03 E04 E05 E06 E07 E08 E09 E10 E11 E12 E13 E14 E15 E16 E17 E18 E19 E20"
REQUIRED=${REQUIRED_EDGE_IDS:-"$ALL_IDS"}
missing=0
for id in $REQUIRED; do
  if [ "$id" = "E18" ]; then continue; fi   # covered by CI migrate job, not a Go test
  if ! grep -rq "func TestEdge_${id}_" --include="*_test.go" .; then
    echo "MISSING edge test: $id"; missing=1
  fi
done
exit $missing
