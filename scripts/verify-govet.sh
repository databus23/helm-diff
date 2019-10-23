#!/bin/bash
# script credits : https://github.com/infracloudio/botkube

set -x

go vet . ./cmd/... ./manifest/...
