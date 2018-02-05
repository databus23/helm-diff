HELM_HOME ?= $(shell helm home)
HAS_GLIDE := $(shell command -v glide;)
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)

LDFLAGS := -X main.Version=$(VERSION)

PKG:= github.com/databus23/helm-diff

# Clear the "unreleased" string in BuildMetadata
LDFLAGS += -X $(PKG)/vendor/k8s.io/helm/pkg/version.BuildMetadata=
LDFLAGS += -X $(PKG)/vendor/k8s.io/helm/pkg/version.Version=$(shell grep -A1 "package: k8s.io/helm" glide.yaml | sed -n -e 's/[ ]*version:.*\(v[.0-9]*\).*/\1/p')

.PHONY: install
install: bootstrap build
	mkdir -p $(HELM_HOME)/plugins/helm-diff
	cp diff $(HELM_HOME)/plugins/helm-diff/
	cp plugin.yaml $(HELM_HOME)/plugins/helm-diff/

.PHONY: build
build:
	go build -i -v -o diff -ldflags="$(LDFLAGS)"

.PHONY: bootstrap
bootstrap:
ifndef HAS_GLIDE
	go get -u github.com/Masterminds/glide
endif
	glide install --strip-vendor

.PHONY: dist
dist: export COPYFILE_DISABLE=1 #teach OSX tar to not put ._* files in tar archive
dist:
	rm -rf build/diff/* release/*
	mkdir -p build/diff release/
	cp README.md LICENSE plugin.yaml build/diff
	GOOS=linux GOARCH=amd64 go build -o build/diff/diff -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-linux.tgz diff/
	GOOS=darwin GOARCH=amd64 go build -o build/diff/diff -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-macos.tgz diff/
	rm build/diff/diff
	GOOS=windows GOARCH=amd64 go build -o build/diff/diff.exe -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-diff-windows.tgz diff/

.PHONY: release
release: dist
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	git push
	github-release databus23/helm-diff v$(VERSION) master "v$(VERSION)" "release/*"
