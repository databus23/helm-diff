#!/usr/bin/env bash
# Reproduction test for https://github.com/databus23/helm-diff/issues/1064
#
# Bug: "helm-diff shows `- labels`, if resource contains only
# `app.kubernetes.io/managed-by` label".
#
# A `labels:`/`annotations:` key that is null or empty is semantically
# identical to an absent key, so it must not show up as a diff entry when one
# side renders the (empty) key and the other omits it.
#
# Scenarios (each prints its full diff output to the CI log):
#   A. a chart whose resource has no labels at all
#   B. a release installed with an explicit managed-by label, then diffed
#      against a chart version that dropped the label
#   C. a chart rendering a bare `labels:` (null) key, then a chart version
#      that removed the labels block entirely
#   D. a custom resource (unstructured/CRD path, like the ExternalSecret from
#      the issue) without labels
#   E. flux-style extra labels on the live object
#   F. --take-ownership
#   G. --dry-run=server
#   H. a chart rendering an explicit empty labels map `labels: {}`

set -euo pipefail

ISSUE=1064
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

kubectl create namespace "$NS" 2>/dev/null || true

###############################################################################
# Variant A: chart resource without any labels
###############################################################################
chart "$WORK/a" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-a
data:
  foo: bar
'
helm upgrade -i rel-a "$WORK/a" -n "$NS" >/dev/null
echo "===== live object labels (A) ====="
kubectl get configmap res-a -n "$NS" -o jsonpath='{.metadata.labels}'; echo

run_diff "A plain"      noassert rel-a "$WORK/a" -n "$NS"
run_diff "A three-way"  assert   rel-a "$WORK/a" -n "$NS" --three-way-merge

###############################################################################
# Variant B: release installed with explicit managed-by label, new chart
# version drops the label. The three-way diff must not show any labels noise
# (a real helm upgrade re-adds managed-by, and helm-diff prunes it). The plain
# diff legitimately reports the dropped label, so it is informational only.
###############################################################################
chart "$WORK/b1" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-b
  labels:
    app.kubernetes.io/managed-by: Helm
data:
  foo: bar
'
chart "$WORK/b2" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-b
data:
  foo: bar
'
helm upgrade -i rel-b "$WORK/b1" -n "$NS" >/dev/null
echo "===== live object labels (B) ====="
kubectl get configmap res-b -n "$NS" -o jsonpath='{.metadata.labels}'; echo

run_diff "B plain"     noassert rel-b "$WORK/b2" -n "$NS"
run_diff "B three-way" assert   rel-b "$WORK/b2" -n "$NS" --three-way-merge

###############################################################################
# Variant C: chart renders a bare `labels:` (null) key, new chart version
# removes the labels block entirely.
###############################################################################
chart "$WORK/c1" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-c
  labels:
data:
  foo: bar
'
chart "$WORK/c2" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-c
data:
  foo: bar
'
helm upgrade -i rel-c "$WORK/c1" -n "$NS" >/dev/null
echo "===== live object labels (C) ====="
kubectl get configmap res-c -n "$NS" -o jsonpath='{.metadata.labels}'; echo

run_diff "C plain"     assert   rel-c "$WORK/c2" -n "$NS"
run_diff "C three-way" assert   rel-c "$WORK/c2" -n "$NS" --three-way-merge

###############################################################################
# Variant D: custom resource (unstructured / merge-patch path) without labels,
# mirroring the ExternalSecret from the issue.
###############################################################################
cat <<'YAML' | kubectl apply -f - >/dev/null
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.issue1064.com
spec:
  group: issue1064.com
  names:
    kind: Widget
    plural: widgets
    singular: widget
    listKind: WidgetList
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              foo:
                type: string
YAML
kubectl wait --for=condition=Established crd/widgets.issue1064.com --timeout=120s >/dev/null

chart "$WORK/d" 'apiVersion: issue1064.com/v1
kind: Widget
metadata:
  name: res-d
  finalizers:
  - issue1064.com/cleanup
spec:
  foo: bar
'
helm upgrade -i rel-d "$WORK/d" -n "$NS" >/dev/null
echo "===== live object labels (D) ====="
kubectl get widget res-d -n "$NS" -o jsonpath='{.metadata.labels}'; echo

run_diff "D plain"     noassert rel-d "$WORK/d" -n "$NS"
run_diff "D three-way" assert   rel-d "$WORK/d" -n "$NS" --three-way-merge

###############################################################################
# Variant E: flux-style labels on the live object (stripped since the fluxcd
# fix), on top of the managed-by label.
###############################################################################
kubectl label configmap res-a -n "$NS" \
  helm.toolkit.fluxcd.io/name=rel-a \
  helm.toolkit.fluxcd.io/namespace="$NS" \
  --overwrite >/dev/null
echo "===== live object labels (E) ====="
kubectl get configmap res-a -n "$NS" -o jsonpath='{.metadata.labels}'; echo

run_diff "E plain"     noassert rel-a "$WORK/a" -n "$NS"
run_diff "E three-way" assert   rel-a "$WORK/a" -n "$NS" --three-way-merge

###############################################################################
# Variant F: --take-ownership path (ParseObject prunes live objects too)
###############################################################################
echo "===== live object labels (F: same as A) ====="
run_diff "F take-ownership" assert rel-a "$WORK/a" -n "$NS" --take-ownership

###############################################################################
# Variant G: --dry-run=server template mode against variant B charts
###############################################################################
run_diff "G dry-run-server" assert rel-b "$WORK/b2" -n "$NS" --dry-run=server --three-way-merge
run_diff "G2 dry-run-server plain" noassert rel-b "$WORK/b2" -n "$NS" --dry-run=server

###############################################################################
# Variant H: chart renders an explicit empty labels map `labels: {}`
###############################################################################
chart "$WORK/h1" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-h
  labels: {}
data:
  foo: bar
'
chart "$WORK/h2" 'apiVersion: v1
kind: ConfigMap
metadata:
  name: res-h
data:
  foo: bar
'
helm upgrade -i rel-h "$WORK/h1" -n "$NS" >/dev/null
echo "===== live object labels (H) ====="
kubectl get configmap res-h -n "$NS" -o jsonpath='{.metadata.labels}'; echo

run_diff "H plain"     assert   rel-h "$WORK/h2" -n "$NS"
run_diff "H three-way" assert   rel-h "$WORK/h2" -n "$NS" --three-way-merge

###############################################################################
finish
