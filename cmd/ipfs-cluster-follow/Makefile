# go source files
SRC := $(shell find ../.. -type f -name '*.go')
GOPATH := $(shell go env GOPATH)
GOFLAGS := -trimpath -buildvcs=false
LDFLAGS ?= -X main.commit=$(shell git rev-parse HEAD 2>/dev/null || echo "")

all: ipfs-cluster-follow

ipfs-cluster-follow: $(SRC)
	go build $(GOFLAGS) -mod=readonly -ldflags "$(LDFLAGS)"

build: ipfs-cluster-follow

install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)"

clean:
	rm -f ipfs-cluster-follow

.PHONY: clean install build
