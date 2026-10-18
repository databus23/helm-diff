#!/usr/bin/env bash
# Reproduction test for https://github.com/databus23/helm-diff/issues/464
#
# Bug: "Diff is incorrect when values in values.yaml change and using local Chart".
#
# When no --set/--values flag is given, helm-diff fetched the values of the
# release with `helm get values --all`, i.e. the fully coalesced values
# including the values.yaml of the chart the release was installed from,
# and passed them to the render command as --values. A --values file takes
# precedence over the new chart's values.yaml, so every change to a chart
# default was masked and the diff came out empty.
#
# Helm's own default only reuses the user-supplied values of the release
# (current.Config), never the coalesced ones, so chart default changes do
# apply. `helm upgrade --reuse-values` is the flag that keeps the old chart
# values.
#
# Scenarios (each prints its full diff output to the CI log):
#   A. chart default change, release without user-supplied values:
#      the change is visible in the diff
#   B. same release with --reuse-values: the old chart values are kept,
#      so the change is not visible (unchanged --reuse-values semantics)
#   C. chart default change, release with a user-supplied value:
#      the user value wins, no diff
#   D. same release with --set: the release values are not reused,
#      the --set value replaces the user value in the diff

set -euo pipefail

ISSUE=464
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

# expect <scenario> <description> <ERE pattern> <present|absent>
expect() {
  local scenario="$1" desc="$2" pattern="$3" want="$4" got
  local out="$WORK/${scenario// /_}.out"
  strip_ansi < "$out" > "$WORK/stripped.out"
  if grep -qE "$pattern" "$WORK/stripped.out"; then
    got=present
  else
    got=absent
  fi
  if [ "$got" = "$want" ]; then
    echo "OK: [$scenario] $desc is $want"
  else
    echo "FAIL: [$scenario] expected $desc to be $want (#${ISSUE})"
    FAIL=1
  fi
}

# new_chart <dir> <default> writes a chart with a single ConfigMap whose
# only value comes from the chart's values.yaml.
new_chart() {
  mkdir -p "$1/templates"
  cat > "$1/Chart.yaml" <<'YAML'
apiVersion: v2
name: issue-test
version: 0.1.0
YAML
  printf 'key: %s\n' "$2" > "$1/values.yaml"
  cat > "$1/templates/cm.yaml" <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
data:
  key: {{ .Values.key | quote }}
YAML
}

kubectl create namespace "$NS" 2>/dev/null || true

new_chart "$WORK/chart" old
# rel has no user-supplied values, rel2 has one that shadows the default.
helm upgrade -i rel "$WORK/chart" -n "$NS" >/dev/null
helm upgrade -i rel2 "$WORK/chart" -n "$NS" --set key=user >/dev/null

# check <description> <got> <want> guards a setup assumption and fails the
# test on mismatch.
check() {
  local desc="$1" got="$2" want="$3"
  if [ "$got" = "$want" ]; then
    echo "OK: $desc"
  else
    echo "FAIL: $desc, got: $got, want: $want (#${ISSUE})"
    # shellcheck disable=SC2034 # FAIL is read by finish() in lib.sh
    FAIL=1
  fi
}

# `helm get values` must print exactly what the scenarios below rely on.
check "rel has no user-supplied values" "$(helm get values rel -n "$NS" --output yaml)" "null"
check "rel2 has key=user as its only user-supplied value" "$(helm get values rel2 -n "$NS" --output yaml)" "key: user"

# The only change from here on is the chart's default in values.yaml.
printf 'key: new\n' > "$WORK/chart/values.yaml"

###############################################################################
# Variant A: the chart default change must be visible in the default diff
###############################################################################
run_diff "A default" noassert rel "$WORK/chart" -n "$NS"
expect "A default" "the new chart default (key: \"new\")" '^\+[[:space:]]*key: "new"' present
expect "A default" "the old chart default (key: \"old\")" '^-[[:space:]]*key: "old"' present

###############################################################################
# Variant B: --reuse-values keeps the values of the chart the release was
# installed from, so the change stays invisible (as in plain Helm)
###############################################################################
run_diff "B reuse-values" noassert rel "$WORK/chart" -n "$NS" --reuse-values
expect "B reuse-values" "no key diff (--reuse-values keeps the old chart values)" '^[+-][[:space:]]*key:' absent

###############################################################################
# Variant C: a user-supplied value shadows the default, so the default change
# does not affect the diff
###############################################################################
run_diff "C user-value" noassert rel2 "$WORK/chart" -n "$NS"
expect "C user-value" "no key diff (the user-supplied value wins)" '^[+-][[:space:]]*key:' absent

###############################################################################
# Variant D: with --set, Helm does not reuse the values of the release,
# so the --set value replaces the user value
###############################################################################
run_diff "D set-override" noassert rel2 "$WORK/chart" -n "$NS" --set key=override
expect "D set-override" "the old user value (key: \"user\")" '^-[[:space:]]*key: "user"' present
expect "D set-override" "the new value (key: \"override\")" '^\+[[:space:]]*key: "override"' present

finish
