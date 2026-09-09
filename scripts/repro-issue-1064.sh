#!/usr/bin/env bash
# Reproduction script for https://github.com/databus23/helm-diff/issues/1064
#
# Bug: "helm-diff shows `- labels`, if resource contains only
# `app.kubernetes.io/managed-by` label".
#
# After the managed-by label is pruned from the diff, the leftover empty (or
# null) `labels` key must not show up as a confusing `- labels:` diff entry.
#
# This script exercises several chart shapes against a real cluster:
#   A. a chart whose resource has no labels at all
#   B. a release installed with an explicit managed-by label, then diffed
#      against a chart version that dropped the label
#   C. a chart rendering a bare `labels:` (null) key, then a chart version
#      that removed the labels block entirely
#   D. a custom resource (unstructured/CRD path, like the ExternalSecret from
#      the issue) without labels
#   E. flux-style extra labels on the live object
#
# Every helm diff invocation prints its full output so the CI log shows the
# actual behavior. Plain (text) diffs are informational: a chart that really
# dropped a labels block legitimately shows a textual change. The assertion
# targets three-way-merge diffs, which fetch live objects and must not report
# any labels-only change (a labels key with no content left after pruning).

set -euo pipefail

NS="d1064"
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
    echo "FAIL: scenario [$scenario] shows a labels-only diff (#1064):"
    echo "$symptoms"
    FAIL=1
  else
    echo "OK: scenario [$scenario] has no labels-only diff"
  fi
}

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

chart() { # chart <dir> <template-content>
  mkdir -p "$1/templates"
  cat > "$1/Chart.yaml" <<'YAML'
apiVersion: v2
name: issue1064
version: 0.1.0
YAML
  cat > "$1/templates/res.yaml" <<< "$2"
}

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
# (a real helm upgrade re-adds managed-by, and helm-diff prunes it).
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
echo ""
if [ "$FAIL" -ne 0 ]; then
  echo "issue 1064 reproduced: labels-only diff entries found"
  exit 1
fi
echo "no labels-only diff entries found"
