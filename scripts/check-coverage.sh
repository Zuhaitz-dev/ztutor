#!/usr/bin/env bash
# check-coverage.sh
#
# Runs `go test -cover` and fails if any listed package's coverage is below
# the threshold given in .coverage-thresholds (one "package=percent" per line).
# Packages without an entry are not gated (report-only).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
THRESHOLDS="$ROOT/.coverage-thresholds"

if [ ! -f "$THRESHOLDS" ]; then
	echo "error: $THRESHOLDS not found"
	exit 1
fi

OUT="$(cd "$ROOT" && GOCACHE="$ROOT/.cache/go-build" go test -cover ./internal/... ./cmd/... 2>/dev/null)"

fail=0
while IFS='=' read -r pkg min; do
	# skip blank lines and comments
	case "$pkg" in ""|\#*) continue ;; esac

	cov="$(printf '%s\n' "$OUT" | grep "ok[[:space:]]*ztutor/$pkg[[:space:]]" | sed -E 's/.*coverage: ([0-9.]+)% of statements.*/\1/' | head -1)"
	if [ -z "$cov" ]; then
		echo "WARN: no coverage reported for $pkg (no tests?)"
		continue
	fi
	if awk -v c="$cov" -v m="$min" 'BEGIN { exit (c < m) ? 0 : 1 }'; then
		echo "FAIL: $pkg coverage ${cov}% < ${min}%"
		fail=1
	else
		echo "ok:   $pkg coverage ${cov}% (>= ${min}%)"
	fi
done < "$THRESHOLDS"

if [ "$fail" -eq 1 ]; then
	echo "coverage gate failed — raise coverage above the .coverage-thresholds values."
	exit 1
fi
echo "coverage gate passed"
