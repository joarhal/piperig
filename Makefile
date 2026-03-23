.PHONY: build test vet generate screenshots clean

build:
	go generate ./...
	go build -o piperig ./cmd/piperig/

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
