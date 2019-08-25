#!/bin/bash
# script credits : https://github.com/infracloudio/botkube

set -x

go vet ./pkg/...
go vet ./cmd/...
