HELM_HOME ?= $(shell helm home)
HAS_GLIDE := $(shell command -v glide;)
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)

PKG:= github.com/databus23/helm-diff
LDFLAGS := -X $(PKG)/cmd.Version=$(VERSION)

# Clear the "unreleased" string in BuildMetadata
LDFLAGS += -X $(PKG)/vendor/k8s.io/helm/pkg/version.BuildMetadata=
LDFLAGS += -X $(PKG)/vendor/k8s.io/helm/pkg/version.Version=$(shell grep -A1 "package: k8s.io/helm" glide.yaml | sed -n -e 's/[ ]*version:.*\(v[.0-9]*\).*/\1/p')

.PHONY: format
format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

.PHONY: install
install: build
	mkdir -p $(HELM_HOME)/plugins/helm-diff/bin
	cp bin/diff $(HELM_HOME)/plugins/helm-diff/bin
	cp plugin.yaml $(HELM_HOME)/plugins/helm-diff/

.PHONY: lint
lint:
	scripts/update-gofmt.sh
	scripts/verify-gofmt.sh
	scripts/verify-golint.sh
	scripts/verify-govet.sh

.PHONY: build
build: lint
	mkdir -p bin/
	go build -i -v -o bin/diff -ldflags="$(LDFLAGS)"

.PHONY: test
test:
	go test -v ./...

.PHONY: bootstrap
bootstrap:
ifndef HAS_GLIDE
	go get -u github.com/Masterminds/glide
endif
	glide install --strip-vendor
	command -v golint || go get -u golang.org/x/lint/golint

.PHONY: docker-run-release
docker-run-release: export pkg=/go/src/github.com/databus23/helm-diff
docker-run-release:
	git checkout master
	git push
	docker run -it --rm -e GITHUB_TOKEN -v $(shell pwd):$(pkg) -w $(pkg) golang:1.13.3 make bootstrap release

.PHONY: dist
dist: export COPYFILE_DISABLE=1 #teach OSX tar to not put ._* files in tar archive
dist: export CGO_ENABLED=0
dist:
	rm -rf build/diff/* release/*
	mkdir -p build/diff/bin release/
	cp README.md LICENSE plugin.yaml build/diff
	GOOS=linux GOARCH=amd64 go build -o build/diff/bin/diff -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-linux.tgz diff/
	GOOS=freebsd GOARCH=amd64 go build -o build/diff/bin/diff -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-freebsd.tgz diff/
	GOOS=darwin GOARCH=amd64 go build -o build/diff/bin/diff -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-macos.tgz diff/
	rm build/diff/bin/diff
	GOOS=windows GOARCH=amd64 go build -o build/diff/bin/diff.exe -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-windows.tgz diff/

.PHONY: release
release: lint dist
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	scripts/release.sh v$(VERSION) master
