.PHONY: build test vet generate clean

build:
	go generate ./...
	go build -o piperig ./cmd/piperig/

test:
	go test ./...

vet:
	go vet ./...

generate:
	go generate ./...

clean:
	rm -f piperig
