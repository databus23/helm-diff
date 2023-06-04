HELM_HOME ?= $(shell helm home)
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)

HELM_3_PLUGINS := $(shell helm env HELM_PLUGINS)

PKG:= github.com/databus23/helm-diff/v3
LDFLAGS := -X $(PKG)/cmd.Version=$(VERSION)

GO ?= go

.PHONY: format
format:
	test -z "$$(find . -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

.PHONY: install
install: build
	mkdir -p $(HELM_HOME)/plugins/helm-diff/bin
	cp bin/diff $(HELM_HOME)/plugins/helm-diff/bin
	cp plugin.yaml $(HELM_HOME)/plugins/helm-diff/

.PHONY: install/helm3
install/helm3: build
	mkdir -p $(HELM_3_PLUGINS)/helm-diff/bin
	cp bin/diff $(HELM_3_PLUGINS)/helm-diff/bin
	cp plugin.yaml $(HELM_3_PLUGINS)/helm-diff/

.PHONY: lint
lint:
	scripts/update-gofmt.sh
	scripts/verify-gofmt.sh
	scripts/verify-golint.sh
	scripts/verify-govet.sh

.PHONY: build
build: lint
	mkdir -p bin/
	go build -v -o bin/diff -ldflags="$(LDFLAGS)"

.PHONY: test
test:
	go test -v ./...

.PHONY: bootstrap
bootstrap:
	go mod download
	command -v golint || GO111MODULE=off go get -u golang.org/x/lint/golint

.PHONY: docker-run-release
docker-run-release: export pkg=/go/src/github.com/databus23/helm-diff
docker-run-release:
	git checkout master
	git push
	# needed to avoid "failed to initialize build cache at /.cache/go-build: mkdir /.cache: permission denied"
	mkdir -p docker-run-release-cache
	# uid needs to be set to avoid "error obtaining VCS status: exit status 128"
	# Also, there needs to be a valid Linux user with the uid in the container-
	# otherwise git-push will fail.
	docker build -t helm-diff-release -f Dockerfile.release \
	  --build-arg HELM_DIFF_UID=$(shell id -u) --load .
	docker run -it --rm -e GITHUB_TOKEN \
	-v ${SSH_AUTH_SOCK}:/tmp/ssh-agent.sock -e SSH_AUTH_SOCK=/tmp/ssh-agent.sock \
	-v $(shell pwd):$(pkg) \
	-v $(shell pwd)/docker-run-release-cache:/.cache \
	-w $(pkg) helm-diff-release make bootstrap release

.PHONY: dist
dist: export COPYFILE_DISABLE=1 #teach OSX tar to not put ._* files in tar archive
dist: export CGO_ENABLED=0
dist:
	rm -rf build/diff/* release/*
	mkdir -p build/diff/bin release/
	cp README.md LICENSE plugin.yaml build/diff
	GOOS=linux GOARCH=amd64 $(GO) build -o build/diff/bin/diff -trimpath -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-linux-amd64.tgz diff/
	GOOS=linux GOARCH=arm64 $(GO) build -o build/diff/bin/diff -trimpath -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-linux-arm64.tgz diff/
	GOOS=freebsd GOARCH=amd64 $(GO) build -o build/diff/bin/diff -trimpath -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-freebsd-amd64.tgz diff/
	GOOS=darwin GOARCH=amd64 $(GO) build -o build/diff/bin/diff -trimpath -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-macos-amd64.tgz diff/
	GOOS=darwin GOARCH=arm64 $(GO) build -o build/diff/bin/diff -trimpath -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-macos-arm64.tgz diff/
	rm build/diff/bin/diff
	GOOS=windows GOARCH=amd64 $(GO) build -o build/diff/bin/diff.exe -trimpath -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-windows-amd64.tgz diff/

.PHONY: release
release: lint dist
	scripts/release.sh v$(VERSION)

# Test for the plugin installation with `helm plugin install -v THIS_BRANCH` works
# Useful for verifying modified `install-binary.sh` still works against various environments
.PHONY: test-plugin-installation
test-plugin-installation:
	docker build -f testdata/Dockerfile.install .
