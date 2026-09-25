#!/usr/bin/env bash
# Reproduction test for https://github.com/databus23/helm-diff/issues/917
#
# Bug: "helm diff --three-way-merge does not detect manual changes made to
# Custom Resources (CRDs and CRs) in the cluster".
#
# Unstructured objects fell back to a JSON merge patch built from the old and
# the new release manifest only, so the live object was ignored and manual
# changes (drift) never showed up in the diff.
#
# Scenarios (each prints its full diff output to the CI log):
#   A. --three-way-merge of a clean, unchanged release reports no drift
#   B. manual change on a CR field the chart owns is detected
#      (also without patch permissions, via --three-way-merge-mode client,
#      and NOT detected by a plain diff, which never looks at the cluster)
#   C. a chart change is merged with the manual change, not masked by it
#   D. manual change on the CRD itself is detected
#
# In addition, no scenario may print an error or a diff entry for
# server-managed fields (resourceVersion), which used to break the dry-run.

set -euo pipefail

ISSUE=917
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

kubectl create namespace "$NS" 2>/dev/null || true

CRD_YAML='apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    plural: widgets
    singular: widget
    kind: Widget
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
'

CR_TMPL='apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
spec:
  size: {{ .Values.size | default "small" }}
'

# Helm cannot build a manifest whose CR refers to a CRD that is only defined
# in the same chart ("no matches for kind Widget ... ensure CRDs are installed
# first"), so the CRD has to exist in the cluster before the install. It is
# still part of the release manifest via templates/, which is what scenario D
# relies on.
kubectl apply -f <(printf '%s\n' "$CRD_YAML") >/dev/null
kubectl wait --for=condition=Established crd/widgets.example.com --timeout=60s >/dev/null

chart "$WORK/a" "${CRD_YAML}---
${CR_TMPL}"
helm upgrade -i rel "$WORK/a" -n "$NS" >/dev/null
kubectl get widget w -n "$NS" -o jsonpath='{.spec.size}'; echo " <- live CR size"

###############################################################################
# Variant A: clean release, unchanged chart -> no drift, no errors
###############################################################################
run_diff "A clean" noassert rel "$WORK/a" -n "$NS" --three-way-merge
expect "A clean" "no diff entry for spec.size" '^[+-][[:space:]]*size:' absent
expect "A clean" "no error" '^Error:' absent

###############################################################################
# Variant B: manual change on a CR field the chart owns must be detected
###############################################################################
kubectl patch widget w -n "$NS" --type=merge -p '{"spec":{"size":"large"}}' >/dev/null

run_diff "B drift" noassert rel "$WORK/a" -n "$NS" --three-way-merge
expect "B drift" "diff entry for the live value (size: large)" '^[+-][[:space:]]*size: large' present
expect "B drift" "diff entry for the chart value (size: small)" '^[+-][[:space:]]*size: small' present
expect "B drift" "no error" '^Error:' absent

# A plain diff never looks at the cluster, so it must stay silent about the
# drift (documents the difference the --three-way-merge flag makes).
run_diff "B plain" noassert rel "$WORK/a" -n "$NS"
expect "B plain" "no drift entry without --three-way-merge" '^[+-][[:space:]]*size:' absent

# The local merge (no patch permissions needed) must detect the drift as well.
run_diff "B client" noassert rel "$WORK/a" -n "$NS" --three-way-merge --three-way-merge-mode client
expect "B client" "diff entry for the live value (size: large)" '^[+-][[:space:]]*size: large' present
expect "B client" "diff entry for the chart value (size: small)" '^[+-][[:space:]]*size: small' present
expect "B client" "no error" '^Error:' absent

###############################################################################
# Variant C: chart change merged with the manual change
###############################################################################
run_diff "C chart-change" noassert rel "$WORK/a" -n "$NS" --three-way-merge --set size=medium
expect "C chart-change" "diff entry for the new chart value (size: medium)" '^[+-][[:space:]]*size: medium' present
expect "C chart-change" "no error" '^Error:' absent

###############################################################################
# Variant D: manual change on the CRD itself must be detected
###############################################################################
kubectl patch crd widgets.example.com --type=merge -p '{"spec":{"versions":[{"name":"v1","served":false,"storage":true,"schema":{"openAPIV3Schema":{"type":"object","x-kubernetes-preserve-unknown-fields":true}}}]}}' >/dev/null

run_diff "D crd-drift" noassert rel "$WORK/a" -n "$NS" --three-way-merge
expect "D crd-drift" "diff entry for the live value (served: false)" '^[+-][[:space:]]*served: false' present
expect "D crd-drift" "diff entry for the chart value (served: true)" '^[+-][[:space:]]*served: true' present
expect "D crd-drift" "no error" '^Error:' absent

###############################################################################
# Server-managed fields must never show up as diff entries (the dry-run patch
# used to fail with "metadata.resourceVersion: Invalid value: 0").
###############################################################################
echo ""
for scenario in "A clean" "B drift" "B plain" "B client" "C chart-change" "D crd-drift"; do
  out="$WORK/${scenario// /_}.out"
  strip_ansi < "$out" > "$WORK/stripped.out"
  if grep -qE '^[+-][[:space:]]*resourceVersion:' "$WORK/stripped.out"; then
    echo "FAIL: [$scenario] shows a resourceVersion diff entry (#${ISSUE})"
    FAIL=1
  fi
done
if [ "$FAIL" -eq 0 ]; then
  echo "OK: no resourceVersion diff entries in any scenario"
fi

finish
