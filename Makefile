HELM_HOME ?= $(shell helm env HELM_DATA_HOME)
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)

HELM_PLUGINS := $(shell helm env HELM_PLUGINS)

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

.PHONY: install/helm
install/helm: build
	mkdir -p $(HELM_PLUGINS)/helm-diff/bin
	cp bin/diff $(HELM_PLUGINS)/helm-diff/bin
	cp plugin.yaml $(HELM_PLUGINS)/helm-diff/

.PHONY: lint
lint:
	scripts/update-gofmt.sh
	scripts/verify-gofmt.sh
	scripts/verify-govet.sh

.PHONY: build
build: lint
	mkdir -p bin/
	go build -v -o bin/diff -ldflags="$(LDFLAGS)"

.PHONY: test
test:
	go test -v ./... -coverprofile cover.out -race
	go tool cover -func cover.out

.PHONY: readme
readme: build
	scripts/gen-readme.sh bin/diff

.PHONY: verify-readme
verify-readme: build
	scripts/gen-readme.sh bin/diff
	git diff --exit-code README.md

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
	-w $(pkg) helm-diff-release make release

# dist-package builds the plugin for a single platform and packs it into
# release/$(1).tgz. The archive content is wrapped in a directory named
# after the archive itself (e.g. helm-diff-linux-amd64/). Helm 4's plugin
# installer requires this when installing directly from a tarball: it
# derives the expected directory from the tarball filename and fails with
# "plugin.yaml not found in expected directory" otherwise (see issue #1071).
# Usage: $(call dist-package,<platform>,<GOOS>,<GOARCH>[,<extra env>])
DIST_PACKAGE_FILES := README.md LICENSE plugin.yaml install-binary.sh install-binary.ps1
define dist-package
mkdir -p build/helm-diff-$(1)/bin
cp $(DIST_PACKAGE_FILES) build/helm-diff-$(1)/
goarch=$(3); bin=diff; [ "$(2)" = "windows" ] && bin=$$bin.exe; \
	$(4) GOOS=$(2) GOARCH=$$goarch $(GO) build -o build/helm-diff-$(1)/bin/$$bin -trimpath -ldflags="$(LDFLAGS)"
tar -C build/ -zcvf $(CURDIR)/release/helm-diff-$(1).tgz helm-diff-$(1)/
rm -rf build/helm-diff-$(1)

endef

.PHONY: dist
dist: export COPYFILE_DISABLE=1 #teach OSX tar to not put ._* files in tar archive
dist: export CGO_ENABLED=0
dist:
	rm -rf build/ release/*
	mkdir -p release/
	$(call dist-package,linux-amd64,linux,amd64)
	$(call dist-package,linux-arm64,linux,arm64)
	$(call dist-package,linux-armv6,linux,arm,GOARM=6)
	$(call dist-package,linux-armv7,linux,arm,GOARM=7)
	$(call dist-package,linux-ppc64le,linux,ppc64le)
	$(call dist-package,linux-s390x,linux,s390x)
	$(call dist-package,freebsd-amd64,freebsd,amd64)
	$(call dist-package,macos-amd64,darwin,amd64)
	$(call dist-package,macos-arm64,darwin,arm64)
	$(call dist-package,windows-amd64,windows,amd64)

.PHONY: release
release: lint dist
	scripts/release.sh v$(VERSION)

# Test for the plugin installation with `helm plugin install -v THIS_BRANCH` works
# Useful for verifying modified `install-binary.sh` still works against various environments
.PHONY: test-plugin-installation
test-plugin-installation:
	docker build -f testdata/Dockerfile.install .
