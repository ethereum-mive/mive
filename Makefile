# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: mive all test clean

GOBIN = ./build/bin
GO ?= latest
GORUN = go run

mive:
	$(GORUN) build/ci.go install ./cmd/mive
	@echo "Done building."
	@echo "Run \"$(GOBIN)/mive\" to launch mive."

all:
	$(GORUN) build/ci.go install

test: all
	$(GORUN) build/ci.go test

lint:
	$(GORUN) build/ci.go lint

clean:
	go clean -cache
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go install golang.org/x/tools/cmd/stringer@latest
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install github.com/golang/protobuf/protoc-gen-go@latest
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'
