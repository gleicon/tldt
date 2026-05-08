# tldt Examples

Practical examples demonstrating how to use the tldt library in Go programs.

## Examples

### Basic Summarization

Simple text summarization with algorithm selection.

```bash
cd examples/basic
go run main.go "Your long text here..."

# Or with options
go run main.go -algorithm textrank -sentences 5 "Your text..."

# From file
go run main.go -f article.txt
```

**Key API:** `tldt.Summarize(text, tldt.SummarizeOptions{Algorithm: "ensemble", Sentences: 3})`

### Security Pipeline

Full processing pipeline with PII detection, Unicode sanitization, and injection detection.

```bash
cd examples/pipeline
go run main.go -sanitize -detect-pii -sanitize-pii "Text with potential issues..."
```

**Key API:** `tldt.Pipeline(text, tldt.PipelineOptions{Sanitize: true, SanitizePII: true, ...})`

### OpenAPI Client

Fetches and summarizes OpenAPI/Swagger documentation. Useful for quickly understanding APIs before integrating with them.

```bash
cd examples/openapi-client

# Basic usage
go run main.go -url https://petstore.swagger.io/v2/swagger.json

# With full security pipeline
go run main.go -url https://petstore.swagger.io/v2/swagger.json \
    -sanitize -detect-pii -sanitize-pii -sentences 7

# JSON output for programmatic use
go run main.go -url https://example.com/api/openapi.json -json
```

**Key API:** `tldt.Fetch(url, tldt.FetchOptions{})` followed by `tldt.Pipeline(text, options)`

### HTML Processor

Converts HTML to clean Markdown using readability extraction, then summarizes. Useful for processing web pages from curl or saved HTML files.

```bash
cd examples/html-processor

# Process HTML file
go run main.go -f article.html

# Pipe HTML from curl
curl -s https://example.com/article | go run main.go

# Process with specific sentence count
cat saved_page.html | go run main.go -sentences 3
```

**Key API:** `tldt.ConvertHTML(html, tldt.HTMLConvertOptions{ExtractContent: true, IncludeTitle: true})`

## Running Examples

Each example is self-contained. Navigate to the example directory and run:

```bash
go run main.go [flags] [text...]
```

## Common Patterns

### Summarize Before LLM Context

```go
// Fetch documentation
result, _ := tldt.Fetch(apiDocsURL, tldt.FetchOptions{Timeout: 30 * time.Second})

// Process through security pipeline
pipelineResult, _ := tldt.Pipeline(result.Text, tldt.PipelineOptions{
    Sanitize:    true,   // Clean Unicode
    SanitizePII: true,   // Redact secrets
    Summarize:   tldt.SummarizeOptions{Sentences: 5},
})

// Use in LLM prompt
prompt := fmt.Sprintf("Given this API documentation:\n\n%s\n\nGenerate client code...", 
    pipelineResult.Summary)
```

### Error Handling

```go
result, err := tldt.Fetch(url, tldt.FetchOptions{})
if err != nil {
    if errors.Is(err, tldt.ErrSSRFBlocked) {
        // Handle SSRF block
    }
    if errors.Is(err, tldt.ErrRedirectLimit) {
        // Handle too many redirects
    }
}
```

### Concurrency

All tldt functions are safe for concurrent use:

```go
var wg sync.WaitGroup
for _, doc := range documents {
    wg.Add(1)
    go func(text string) {
        defer wg.Done()
        result, _ := tldt.Summarize(text, tldt.SummarizeOptions{Sentences: 3})
        // Process result
    }(doc)
}
wg.Wait()
```
