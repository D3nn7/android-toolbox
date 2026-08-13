BINARY := android-toolbox
PKG := ./cmd/android-toolbox

VERSION := $(shell cat VERSION 2>/dev/null || echo 0.0.0-dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -X android-toolbox/internal/buildinfo.Version=$(VERSION) -X android-toolbox/internal/buildinfo.Commit=$(COMMIT)

.PHONY: build test vet fmt run clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

run: build
	./bin/$(BINARY)

clean:
	rm -rf bin
