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
