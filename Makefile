.PHONY: build test install clean deps lint run help

BINARY := tldt
CMD     := ./cmd/tldt

## build: compile binary to ./tldt
build:
	go build -o $(BINARY) $(CMD)

## test: run all tests
test:
	go test ./...

## test-verbose: run tests with output
test-verbose:
	go test -v ./...

## install: install binary to GOPATH/bin
install:
	go install $(CMD)

## deps: tidy and verify modules
deps:
	go mod tidy
	go mod verify

## clean: remove compiled binary
clean:
	rm -f $(BINARY)

## lint: run go vet
lint:
	go vet ./...

## run: build and run with stdin (usage example)
run: build
	@echo "Built. Pipe text: echo 'your text' | ./$(BINARY)"

## help: list targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
