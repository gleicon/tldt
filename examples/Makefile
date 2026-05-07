# Makefile for tldt examples
.PHONY: all build clean test basic pipeline openapi help

EXAMPLES := basic pipeline openapi-client

all: build

build: $(EXAMPLES)

basic:
	@echo "Building basic example..."
	cd basic && go build -o basic-example main.go
	@echo "OK: basic-example"

pipeline:
	@echo "Building pipeline example..."
	cd pipeline && go build -o pipeline-example main.go
	@echo "OK: pipeline-example"

openapi:
	@echo "Building openapi-client example..."
	cd openapi-client && go build -o openapi-example main.go
	@echo "OK: openapi-example"

test: build
	@echo ""
	@echo "Testing basic example..."
	cd basic && ./basic-example -sentences 2 "The quick brown fox jumps over the lazy dog. Pangrams are sentences containing all letters."
	@echo ""
	@echo "Testing pipeline example..."
	cd pipeline && ./pipeline-example -detect-pii "Contact test@example.com for info."
	@echo ""
	@echo "All tests passed."

clean:
	@echo "Cleaning build artifacts..."
	-rm -f basic/basic-example
	-rm -f pipeline/pipeline-example
	-rm -f openapi-client/openapi-example
	@echo "Clean complete."

help:
	@echo "tldt Examples Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all       - Build all examples (default)"
	@echo "  build     - Build all examples"
	@echo "  basic     - Build basic summarization example"
	@echo "  pipeline  - Build security pipeline example"
	@echo "  openapi   - Build OpenAPI client example"
	@echo "  test      - Build and run all examples"
	@echo "  clean     - Remove built binaries"
	@echo "  help      - Show this help"
	@echo ""
	@echo "Usage:"
	@echo "  make        - Build all examples"
	@echo "  make test   - Build and test all examples"
	@echo "  make clean  - Clean build artifacts"
