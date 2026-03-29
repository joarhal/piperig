.PHONY: build test vet generate screenshots clean

VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD)
DATE    := $(shell date -u +%Y-%m-%d)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go generate ./...
	go build -ldflags "$(LDFLAGS)" -o piperig ./cmd/piperig/

test:
	go test ./...

vet:
	go vet ./...

generate:
	go generate ./...

screenshots:
	./assets/generate.sh

clean:
	rm -f piperig
