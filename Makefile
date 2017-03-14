HELM_HOME ?= $(shell helm home)
HAS_GLIDE := $(shell command -v glide;)
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)

LDFLAGS := -X main.Version=$(VERSION)

.PHONY: install
install: bootstrap build
	mkdir -p $(HELM_HOME)/plugins/diff
	cp diff $(HELM_HOME)/plugins/diff/
	cp plugin.yaml $(HELM_HOME)/plugins/diff/

.PHONY: build
build:
	go build -o diff -ldflags="$(LDFLAGS)"

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
	tar -C build/ -zcvf $(CURDIR)/release/helm-template-linux.tgz diff/
	GOOS=darwin GOARCH=amd64 go build -o build/diff/diff -ldflags="$(LDFLAGS)"
	tar -C build/ -zcvf $(CURDIR)/release/helm-template-macos.tgz diff/

.PHONY: release
release: dist
ifndef GITHUB_ACCESS_TOKEN
	$(error GITHUB_ACCESS_TOKEN is undefined)
endif
	git push
	gh-release create databus23/helm-diff $(VERSION) master
