#!/usr/bin/env bash
# Run the backend tests inside the container (Windows App Control blocks *.test.exe on the dev machine).
# Usage: scripts/test-backend.sh [go test args...]   (default: ./... -count=1 -p 1)
#
# A fresh clone has no internal/store/gen (sqlc output is not committed), so generate it first with the
# same sqlc version CI uses. Integration packages share one database and pump one outbox, so they must
# not run in parallel: -p 1 is the default unless you pass your own -p.
set -uo pipefail
cd /src
if [ ! -d internal/store/gen ] || [ -z "$(ls -A internal/store/gen 2>/dev/null)" ]; then
  echo "internal/store/gen is missing: running sqlc generate" >&2
  go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 >/dev/null 2>&1 || { echo "cannot install sqlc" >&2; exit 2; }
  "$(go env GOPATH)/bin/sqlc" generate --no-remote || { echo "sqlc generate failed" >&2; exit 2; }
fi
ARGS=("$@"); [ ${#ARGS[@]} -eq 0 ] && ARGS=(./... -count=1)
case " ${ARGS[*]} " in *" -p "*|*" -p="*) ;; *) ARGS+=(-p 1) ;; esac
go test "${ARGS[@]}" 2>&1