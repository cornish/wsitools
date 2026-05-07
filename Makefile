.PHONY: test vet cover bench install clean

GO ?= go
BIN = bin/wsi-tools

test:
	$(GO) test ./... -race -count=1

vet:
	$(GO) vet ./...

cover:
	$(GO) test ./... -race -count=1 -coverprofile=coverage.txt -covermode=atomic
	$(GO) tool cover -func=coverage.txt | tail -1

bench:
	$(GO) test ./tests/bench/... -bench=. -benchmem -run=^$$

install:
	$(GO) install ./cmd/wsi-tools

build:
	$(GO) build -o $(BIN) ./cmd/wsi-tools

clean:
	rm -rf bin/ coverage.txt
