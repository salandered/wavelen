#!/usr/bin/env bash
# -- Smoke test --
# Waits for the composed stack and asserts the readiness endpoint serves 200.
# /readyz checks that the HTTP server can reach Postgres.
#
# The version endpoint is checked, so a broken -X version injection
# would fail.

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
attempts="${ATTEMPTS:-60}"

for i in $(seq 1 "$attempts"); do
	code="$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/readyz" || true)"
	if [ "$code" = "200" ]; then
		echo "SMOKE OK: $BASE_URL/readyz -> 200"
		version="$(curl -s "$BASE_URL/api/v1/version")"
		case "$version" in
		*'"version":'*)
			echo "SMOKE OK: $BASE_URL/api/v1/version -> $version"
			exit 0
			;;
		esac
		echo "SMOKE FAIL: $BASE_URL/api/v1/version is not a version: $version" >&2
		exit 1
	fi
	echo "waiting for $BASE_URL/readyz (attempt $i/$attempts, last=$code)"
	sleep 2
done

echo "SMOKE FAIL: $BASE_URL/readyz did not return 200 after $attempts attempts" >&2
exit 1
