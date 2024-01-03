#!/bin/bash
# script credits : https://github.com/infracloudio/botkube

set -o nounset
set -o pipefail

find_packages() {
  find . -not \( \
      \( \
        -wholename '*/vendor/*' \
      \) -prune \
    \) -name '*.go' -exec dirname '{}' ';' | sort -u
}

errors="$(find_packages | xargs -I@ bash -c "staticcheck @")"
if [[ -n "${errors}" ]]; then
  echo "${errors}"
  exit 1
fi
