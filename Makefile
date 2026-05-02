.PHONY: build test test-uat install install-skill clean deps lint run help

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

## test-cover: unit + subprocess coverage report
## Note: -coverprofile and GOCOVERDIR conflict in go test; run in two passes.
COVDIR := $(CURDIR)/covdata
test-cover:
	@mkdir -p $(COVDIR)
	@echo "--- pass 1: unit coverage (all packages) ---"
	go test -count=1 -coverprofile=$(CURDIR)/coverage_unit.out ./...
	@echo ""
	@echo "--- pass 2: subprocess binary coverage (cmd/tldt) ---"
	GOCOVERDIR=$(COVDIR) go test -count=1 ./cmd/tldt/...
	@echo ""
	@echo "=== unit coverage total ==="
	@go tool cover -func=$(CURDIR)/coverage_unit.out | tail -1
	@echo ""
	@echo "=== subprocess (main) coverage ==="
	@go tool covdata func -i=$(COVDIR) | grep "cmd/tldt"
	@rm -rf $(COVDIR) $(CURDIR)/coverage_unit.out

## test-cover-html: open coverage in browser (unit tests only)
test-cover-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

## test-race: run tests with race detector
test-race:
	go test -race ./...

## install: install binary to GOPATH/bin
install:
	go install $(CMD)

## install-skill: install tldt Claude Code skill and UserPromptSubmit hook
install-skill: build
	./$(BINARY) --install-skill

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

## test-uat: run automated UAT tests for phase 4 URL input feature
test-uat:
	@echo "=== UAT: Phase 4 URL Input ==="
	go test -v -count=1 -run 'TestFetch_OK|TestFetch_404|TestFetch_Redirect|TestFetch_InvalidScheme|TestFetch_NonHTMLContentType' ./internal/fetcher/...
	@echo ""
	go test -v -count=1 -run 'TestMain_URLFlag' ./cmd/tldt/...
	@echo ""
	@echo "=== UAT PASS ==="

## bench: run benchmarks
bench:
	go test -bench=. -benchmem ./...

## help: list targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
