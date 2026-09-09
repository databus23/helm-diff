#!/usr/bin/env bash
# Run issue reproduction tests.
#
# Usage:
#   scripts/issues/run.sh              # run all issue tests
#   scripts/issues/run.sh 1064 [...]   # run specific issue tests
#
# Each test is a script named <issue-number>.sh in this directory. A test
# fails (non-zero exit) when the issue it covers is reproduced; all output is
# streamed to stdout/stderr so CI logs show the full diff behavior.

set -uo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"

if [ "$#" -gt 0 ]; then
  tests=("$@")
else
  tests=()
  for f in "$DIR"/[0-9]*.sh; do
    [ -e "$f" ] || continue
    tests+=("$(basename "$f" .sh)")
  done
fi

if [ "${#tests[@]}" -eq 0 ]; then
  echo "no issue tests found in $DIR"
  exit 1
fi

overall=0
summary=()

for issue in "${tests[@]}"; do
  script="$DIR/$issue.sh"
  if [ ! -e "$script" ]; then
    echo "ERROR: no test for issue #$issue ($script)"
    overall=1
    summary+=("#$issue MISSING")
    continue
  fi
  echo ""
  echo "#######################################################################"
  echo "# Running issue test #$issue"
  echo "#######################################################################"
  if bash "$script"; then
    summary+=("#$issue PASS")
  else
    overall=1
    summary+=("#$issue FAIL")
  fi
done

echo ""
echo "#######################################################################"
echo "# Issue test summary"
echo "#######################################################################"
printf '%s\n' "${summary[@]}"

exit "$overall"
