#!/bin/bash
# script credits : https://github.com/infracloudio/botkube

set -o errexit
set -o nounset
set -o pipefail

find_files() {
  find . -not \( \
      \( \
        -wholename '*/vendor/*' \
      \) -prune \
    \) -name '*.go'
}

bad_files=$(find_files | xargs -I@ bash -c "golint @")
if [[ -n "${bad_files}" ]]; then
  echo "${bad_files}"
  exit 1
fi
