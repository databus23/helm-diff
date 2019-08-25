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

find_files | xargs gofmt -w -s
