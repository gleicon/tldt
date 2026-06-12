.PHONY: build test test-uat test-injection install install-skill clean deps lint run help wasm demo demo-build wasm-clean

BINARY := tldt
CMD     := ./cmd/tldt
WASM_DIR := ./wasm
DOCS_DIR := ./docs
GOROOT   := $(shell go env GOROOT)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

## build: compile binary to ./tldt
build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

## test: run all tests including injection surface detection
test: test-injection
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

## clean: remove compiled binary and WASM files
clean:
	rm -f $(BINARY)
	rm -f $(DOCS_DIR)/tldt.wasm $(DOCS_DIR)/wasm_exec.js

## lint: run go vet and golangci-lint
lint:
	go vet ./...
	golangci-lint run ./...

## run: build and run with stdin (usage example)
run: build
	@echo "Built. Pipe text: echo 'your text' | ./$(BINARY)"

## test-injection: run document surface injection detection tests (PDF, DOCX, XLSX, HTML)
test-injection:
	@echo "=== Injection surface detection tests ==="
	go test -v -count=1 -run 'TestDetectInjection|TestExtractDOCX|TestExtractXLSX|TestExtractPDF|TestExtractHTMLSurfaces' ./...
	@echo ""
	@echo "=== PASS ==="

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

## wasm: build WebAssembly binary for browser demo
wasm:
	@echo "Building WASM binary..."
	GOOS=js GOARCH=wasm go build -o $(DOCS_DIR)/tldt.wasm $(WASM_DIR)

## wasm-exec: copy wasm_exec.js runtime to docs
wasm-exec:
	@echo "Copying wasm_exec.js..."
	@if [ -f "$(GOROOT)/misc/wasm/wasm_exec.js" ]; then \
		cp "$(GOROOT)/misc/wasm/wasm_exec.js" $(DOCS_DIR)/wasm_exec.js; \
	elif [ -f "$(GOROOT)/lib/wasm/wasm_exec.js" ]; then \
		cp "$(GOROOT)/lib/wasm/wasm_exec.js" $(DOCS_DIR)/wasm_exec.js; \
	else \
		echo "ERROR: wasm_exec.js not found in $(GOROOT)"; \
		exit 1; \
	fi

## demo-build: build WASM and copy runtime files
demo-build: wasm wasm-exec
	@echo "Demo build complete. Files in $(DOCS_DIR)/:"
	@ls -lh $(DOCS_DIR)/tldt.wasm $(DOCS_DIR)/wasm_exec.js 2>/dev/null || echo "  (check files)"

## demo: build demo and serve locally for testing
demo: demo-build
	@echo "Starting local server for demo..."
	@echo "Open: http://localhost:8080/demo.html"
	@cd $(DOCS_DIR) && python3 -m http.server 8080 2>/dev/null || python -m SimpleHTTPServer 8080 2>/dev/null || (echo "Install Python or serve docs/ manually" && exit 1)

## wasm-clean: remove WASM build artifacts
wasm-clean:
	rm -f $(DOCS_DIR)/tldt.wasm $(DOCS_DIR)/wasm_exec.js

## release: tag a release (usage: make release VERSION=v1.0.0)
release:
	@test -n "$(VERSION)" || (echo "usage: make release VERSION=vX.Y.Z" && exit 1)
	@git diff --quiet HEAD || (echo "uncommitted changes — commit first" && exit 1)
	@echo "Tagging $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "Push with: git push origin $(VERSION)"

## help: list targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
