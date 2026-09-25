#!/usr/bin/env bash
# Shared helpers for issue reproduction tests.
#
# Each issue test lives in scripts/issues/<issue-number>.sh and is executed
# against a real cluster (see scripts/issues/run.sh and the CI integration
# job). Source this file from an issue test to get the helpers below.

# The issue test must define ISSUE before sourcing this file. It is used for
# the namespace and the log/output prefix.
if [ -z "${ISSUE:-}" ]; then
  echo "lib.sh: ISSUE must be set to the issue number" >&2
  exit 1
fi

NS="issue-${ISSUE}"
WORK="$(mktemp -d)"
FAIL=0

strip_ansi() {
  sed 's/\x1b\[[0-9;]*m//g'
}

# Detect labels-only diff entries: a +/- line whose payload is `labels:`,
# `labels: null` or `labels: {}` and whose following line is not an indented
# child entry (which would make it a legitimate multi-line label change).
find_symptoms() {
  awk '
    { lines[NR] = $0 }
    END {
      for (i = 1; i <= NR; i++) {
        line = lines[i]
        if (line !~ /^[+-][[:space:]]*labels:([[:space:]]+(null|\{\}))?[[:space:]]*$/)
          continue
        payload = line
        sub(/^[+-]/, "", payload)
        val = payload
        sub(/^[[:space:]]*labels:/, "", val)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
        if (val == "null" || val == "{}") {
          printf "  line %d: %s\n", i, line
          continue
        }
        # bare `labels:` key: only a symptom when no child entry follows
        match(payload, /^[[:space:]]*/); curind = RLENGTH
        nxt = lines[i + 1]
        if (nxt != "") {
          nprefix = substr(nxt, 1, 1)
          nrest = substr(nxt, 2)
          if (nprefix == "+" || nprefix == "-" || nprefix == " ") {
            match(nrest, /^[[:space:]]*/)
            if (RLENGTH > curind)
              continue
          }
        }
        printf "  line %d: %s\n", i, line
      }
    }' "$1"
}

check_no_symptom() {
  local output_file="$1" scenario="$2"
  strip_ansi < "$output_file" > "$WORK/stripped.out"
  local symptoms
  symptoms="$(find_symptoms "$WORK/stripped.out")"
  if [ -n "$symptoms" ]; then
    echo "FAIL: scenario [$scenario] shows a labels-only diff (#${ISSUE}):"
    echo "$symptoms"
    FAIL=1
  else
    echo "OK: scenario [$scenario] has no labels-only diff"
  fi
}

# run_diff <scenario> <assert|noassert> <helm diff args...>
# Prints the full diff output (so CI logs show the actual behavior) and,
# when asserted, fails the test on labels-only diff entries.
run_diff() {
  local scenario="$1" assert="$2"; shift 2
  local out="$WORK/${scenario// /_}.out"
  echo ""
  echo "===== helm diff $* [$scenario] ====="
  set +e
  helm diff upgrade "$@" > "$out" 2>&1
  local rc="$?"
  set -e
  strip_ansi < "$out"
  echo "----- exit code: $rc -----"
  if [ "$assert" = "assert" ]; then
    check_no_symptom "$out" "$scenario"
  fi
}

# chart <dir> <template-content> writes a minimal chart with a single
# templates/res.yaml rendered from the given content.
chart() {
  mkdir -p "$1/templates"
  cat > "$1/Chart.yaml" <<'YAML'
apiVersion: v2
name: issue-test
version: 0.1.0
YAML
  cat > "$1/templates/res.yaml" <<< "$2"
}

finish() {
  echo ""
  if [ "$FAIL" -ne 0 ]; then
    echo "issue #${ISSUE}: reproduction detected (FAILED)"
    exit 1
  fi
  echo "issue #${ISSUE}: all scenarios passed"
}
