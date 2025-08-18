# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go linter (`github.com/ovargas/wrap-error-linter`) that enforces proper error wrapping patterns in Go code. It ensures errors from external packages are wrapped with context before being returned.

## Current Implementation Status

The linter has been fully implemented with the following features:
- ✅ Detects unwrapped errors from external packages
- ✅ Identifies double wrapping within the same package
- ✅ Warns about using `%v` instead of `%w` for error wrapping
- ✅ Configurable context requirements for error messages
- ✅ Multiple output formats (text, JSON, XML, HTML)
- ✅ Configuration file support (.wrap-error-linter.yml)
- ✅ golangci-lint compatible
- ✅ Recognizes sentinel errors and custom error wrappers

## Project Structure

```
wrap-error-linter/
├── cmd/wrap-error-linter/     # CLI entry point
├── pkg/
│   ├── analyzer/              # Core linting logic
│   ├── config/                # Configuration handling
│   └── reporter/              # Output formatters
├── internal/
│   ├── astutil/               # AST helper functions
│   └── pkgutil/               # Package analysis utilities
└── testdata/                  # Test cases
```

## Build and Run Commands

```bash
# Build the linter
go build -o wrap-error-linter ./cmd/wrap-error-linter

# Run the linter (using go/analysis singlechecker)
./wrap-error-linter ./...

# Run with specific package
./wrap-error-linter ./pkg/...

# Run tests
go test ./...

# Install globally
go install github.com/ovargas/wrap-error-linter/cmd/wrap-error-linter@latest
```

## Configuration

The linter uses `.wrap-error-linter.yml` for configuration. Key settings:
- `mode`: warn or fail
- `output`: text, json, xml, or html
- `require-context`: enforce descriptive error messages
- `severity`: per-rule severity levels
- `custom-wrappers`: register custom error wrapping functions
- `trusted-packages`: internal packages that don't require wrapping

## Known Limitations

1. The linter works best when analyzing complete packages rather than individual files
2. Type information is required for accurate analysis, so the code must compile
3. The singlechecker mode requires packages to be properly structured Go modules

## Integration with golangci-lint

Add to `.golangci.yml`:
```yaml
linters-settings:
  custom:
    wrap-error:
      type: "module"
      original-url: "github.com/ovargas/wrap-error-linter"

linters:
  enable:
    - wrap-error
```

## Testing Approach

Use the `golang.org/x/tools/go/analysis/analysistest` framework for testing the analyzer with test data in the `testdata/` directory.