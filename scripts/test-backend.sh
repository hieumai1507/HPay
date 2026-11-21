#!/usr/bin/env bash
# Chay go test cho backend trong container (may Windows co App Control chan *.test.exe).
# Dung: scripts/test-backend.sh [go test args...]   (mac dinh: ./... -count=1)
set -uo pipefail
cd /src
ARGS=("$@"); [ ${#ARGS[@]} -eq 0 ] && ARGS=(./... -count=1)
go test "${ARGS[@]}" 2>&1