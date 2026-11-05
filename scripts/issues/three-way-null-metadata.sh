#!/usr/bin/env bash
# Reproduction test: --three-way-merge reports the removal of every live
# annotation (or label) when the chart renders the map as null.
#
# A chart that renders `annotations:` with nothing under it produces a null map.
# helm upgrade merges its ownership metadata (meta.helm.sh/release-name and
# meta.helm.sh/release-namespace, app.kubernetes.io/managed-by) into every
# object before it computes the three-way patch, so the map it sends is never
# null and annotations added outside the chart survive the upgrade. helm-diff
# must compute the same patch and report no change.
#
# Scenarios (each prints its full diff output to the CI log):
#   A. null annotations, live object annotated outside the chart
#   B. null labels, live object labelled outside the chart
#   C. A with --three-way-merge-mode client
#   D. an annotation the chart sets, edited outside the chart, is still drift

set -euo pipefail

ISSUE=three-way-null-metadata
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

kubectl create namespace "$NS" 2>/dev/null || true

# expect_exit <scenario> <expected exit code> <helm diff args...> fails the
# test unless helm diff --detailed-exitcode exits with the expected code.
expect_exit() {
  local scenario="$1" want="$2"; shift 2
  local out="$WORK/${scenario// /_}.out"
  echo ""
  echo "===== helm diff $* [$scenario] ====="
  set +e
  helm diff upgrade --detailed-exitcode "$@" > "$out" 2>&1
  local rc="$?"
  set -e
  strip_ansi < "$out"
  echo "----- exit code: $rc -----"
  if [ "$rc" -ne "$want" ]; then
    echo "FAIL: scenario [$scenario] exited $rc, want $want"
    FAIL=1
  else
    echo "OK: scenario [$scenario] exited $want"
  fi
}

###############################################################################
# Variant A: null annotations, annotation added outside the chart
###############################################################################
chart "$WORK/a" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-a
  annotations:
data:
  foo: bar
'
helm upgrade -i rel-a "$WORK/a" -n "$NS" >/dev/null
kubectl annotate configmap res-a -n "$NS" example.com/added-outside=yes --overwrite >/dev/null

expect_exit "A three-way" 0 rel-a "$WORK/a" -n "$NS" --three-way-merge

###############################################################################
# Variant B: null labels, label added outside the chart
###############################################################################
chart "$WORK/b" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-b
  labels:
data:
  foo: bar
'
helm upgrade -i rel-b "$WORK/b" -n "$NS" >/dev/null
kubectl label configmap res-b -n "$NS" example.com/added-outside=yes --overwrite >/dev/null

expect_exit "B three-way" 0 rel-b "$WORK/b" -n "$NS" --three-way-merge

###############################################################################
# Variant C: variant A, merged locally
###############################################################################
expect_exit "C three-way client" 0 rel-a "$WORK/a" -n "$NS" --three-way-merge --three-way-merge-mode client

###############################################################################
# Variant D: the chart sets an annotation that is edited outside the chart
###############################################################################
chart "$WORK/d" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-d
  annotations:
    example.com/owned-by-chart: chart
data:
  foo: bar
'
helm upgrade -i rel-d "$WORK/d" -n "$NS" >/dev/null
kubectl annotate configmap res-d -n "$NS" example.com/owned-by-chart=edited --overwrite >/dev/null

expect_exit "D three-way drift" 2 rel-d "$WORK/d" -n "$NS" --three-way-merge

###############################################################################
# The upgrade itself keeps what was added outside the chart; this is the
# behavior the diffs above must agree with.
###############################################################################
helm upgrade rel-a "$WORK/a" -n "$NS" >/dev/null
helm upgrade rel-b "$WORK/b" -n "$NS" >/dev/null
for check in "res-a {.metadata.annotations.example\.com/added-outside}" \
             "res-b {.metadata.labels.example\.com/added-outside}"; do
  read -r name path <<< "$check"
  if [ "$(kubectl get configmap "$name" -n "$NS" -o jsonpath="$path")" != "yes" ]; then
    echo "FAIL: helm upgrade removed the metadata added outside the chart from $name; the premise of this test is wrong"
    FAIL=1
  fi
done

###############################################################################
finish
