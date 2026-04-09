GO ?= go

.PHONY: build test fmt lint run

build:
	$(GO) build ./cmd/sub2clash

test:
	$(GO) test ./...

fmt:
	gofmt -w .

lint:
	test -z "$$(gofmt -l .)"

run:
	$(GO) run ./cmd/sub2clash serve
