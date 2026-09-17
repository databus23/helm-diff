#!/usr/bin/env bash
# Reproduction test for https://github.com/databus23/helm-diff/issues/782
#
# Bug: "Diff failing when diffing helm hook jobs with --take-ownership flag".
#
# Helm only adopts resources from the release manifest. Hooks are kept out of
# it and never get the meta.helm.sh/release-* annotations, so an unchanged hook
# must not be reported as "changed ownership" by --take-ownership.
#
# Scenarios (each prints its full diff output to the CI log):
#   A. plain diff of an unchanged chart with a hook Job (baseline, not checked)
#   B. --take-ownership diff of the same unchanged chart
#   C. --take-ownership still reports a resource that no release owns

set -euo pipefail

ISSUE=782
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

# check_ownership <scenario> <resource name> <reported|not-reported>
check_ownership() {
  local scenario="$1" name="$2" want="$3" got
  local out="$WORK/${scenario// /_}.out"
  strip_ansi < "$out" > "$WORK/stripped.out"
  # A failed diff prints no ownership changes at all, so it must not pass.
  if grep -q '^Error:' "$WORK/stripped.out"; then
    echo "FAIL: scenario [$scenario] helm diff failed (#${ISSUE})"
    FAIL=1
    return
  fi
  if grep -q ", ${name}, .* changed ownership:" "$WORK/stripped.out"; then
    got=reported
  else
    got=not-reported
  fi
  if [ "$got" = "$want" ]; then
    echo "OK: scenario [$scenario] ownership change for $name is $want"
  else
    echo "FAIL: scenario [$scenario] expected ownership change for $name to be $want, got $got (#${ISSUE})"
    FAIL=1
  fi
}

kubectl create namespace "$NS" 2>/dev/null || true

HOOK_CHART='apiVersion: v1
kind: ConfigMap
metadata:
  name: res-a
data:
  foo: bar
---
apiVersion: batch/v1
kind: Job
metadata:
  name: hook-a
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
spec:
  template:
    spec:
      containers:
      - name: hook
        image: busybox:1.36
        command: ["true"]
      restartPolicy: Never
'

chart "$WORK/a" "$HOOK_CHART"
helm upgrade -i rel-a "$WORK/a" -n "$NS" >/dev/null
echo "===== live hook annotations ====="
kubectl get job hook-a -n "$NS" -o jsonpath='{.metadata.annotations}'; echo

###############################################################################
# Variant A: plain diff, logged as a baseline (no ownership check involved)
###############################################################################
run_diff "A plain" noassert rel-a "$WORK/a" -n "$NS"

###############################################################################
# Variant B: --take-ownership must skip the hook
###############################################################################
run_diff "B take-ownership" noassert rel-a "$WORK/a" -n "$NS" --take-ownership
check_ownership "B take-ownership" hook-a not-reported
check_ownership "B take-ownership" res-a not-reported

###############################################################################
# Variant C: a resource created outside Helm is still reported
###############################################################################
kubectl create configmap unowned-c -n "$NS" --from-literal=foo=bar --dry-run=client -o yaml \
  | kubectl apply -f - >/dev/null
chart "$WORK/c" "${HOOK_CHART}---
apiVersion: v1
kind: ConfigMap
metadata:
  name: unowned-c
data:
  foo: bar
"
run_diff "C take-ownership" noassert rel-a "$WORK/c" -n "$NS" --take-ownership
check_ownership "C take-ownership" unowned-c reported
check_ownership "C take-ownership" hook-a not-reported

###############################################################################
finish
