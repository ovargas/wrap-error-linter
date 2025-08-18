# Wrap Error Linter

A Go linter that ensures errors from external packages are properly wrapped with context before being returned.

## Features

- ✅ Enforces error wrapping for external package errors
- ✅ Detects double wrapping within the same package
- ✅ Identifies improper error wrapping (e.g., using `%v` instead of `%w`)
- ✅ Configurable severity levels (ERROR, WARN, INFO)
- ✅ Multiple output formats (text, JSON, XML, HTML)
- ✅ golangci-lint integration
- ✅ Recognizes sentinel errors that don't need wrapping
- ✅ Supports custom error wrapper functions
- ✅ Configurable context requirements for error messages

## Installation

```bash
go install github.com/ovargas/wrap-error-linter/cmd/wrap-error-linter@latest
```

## Usage

### Standalone

```bash
# Analyze current directory
wrap-error-linter ./...

# Analyze specific package
wrap-error-linter ./pkg/...

# Use configuration file
wrap-error-linter -config .wrap-error-linter.yml ./...

# Output to JSON
wrap-error-linter -output json -output-path report.json ./...

# Fail on warnings
wrap-error-linter -mode fail ./...
```

### With golangci-lint

Add to your `.golangci.yml`:

```yaml
linters-settings:
  custom:
    wrap-error:
      type: "module"
      description: "Checks that errors from external packages are properly wrapped"
      original-url: "github.com/ovargas/wrap-error-linter"

linters:
  enable:
    - wrap-error
```

## Configuration

Create a `.wrap-error-linter.yml` file in your project root:

```yaml
# Mode: warn or fail
mode: fail

# Output format: text, json, xml, html
output: text

# Output file path (empty for stdout)
output-path: ""

# Packages to ignore for wrapping requirement
ignore-packages:
  - io
  - database/sql

# Ignore sentinel errors (e.g., io.EOF, sql.ErrNoRows)
ignore-sentinel-errors: true

# Maximum wrapping depth before warning
max-wrap-depth: 10

# Require contextual message when wrapping
require-context: true

# Minimum length for context message
context-min-length: 10

# Required context patterns (regex)
context-patterns:
  - "failed to .+: %w"
  - "unable to .+: %w"
  - "error .+: %w"

# Custom wrapper functions
custom-wrappers:
  packages:
    - package: "github.com/pkg/errors"
      functions: ["Wrap", "Wrapf", "WithStack", "WithMessage"]
    - package: "github.com/mycompany/errors"
      functions: ["WrapContext"]
  auto-detect-unwrap: true

# Severity levels for each rule
severity:
  unwrapped-external-error: error
  double-wrap: warn
  missing-context: warn
  using-percent-v: error
  max-depth-exceeded: warn

# Exclusions
exclude:
  files:
    - "*_test.go"
    - "*.pb.go"
    - "generated_*.go"
  functions:
    - "main"
    - "init"
  packages:
    - "generated"
    - "mocks"
  allow-directives: true  # Support //nolint:wrap-error

# Trusted internal packages that don't require wrapping
trusted-packages:
  - "github.com/mycompany/internal"
  - "github.com/mycompany/pkg"
```

## Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to configuration file | Auto-detect |
| `-mode` | Mode: warn or fail | warn |
| `-output` | Output format: text, json, xml, html | text |
| `-output-path` | Path to output file | stdout |
| `-require-context` | Require context message when wrapping | false |
| `-max-wrap-depth` | Maximum wrapping depth | 10 |
| `-ignore-packages` | Comma-separated packages to ignore | |
| `-trusted-packages` | Comma-separated trusted packages | |
| `-exclude-files` | Comma-separated file patterns to exclude | |
| `-exclude-packages` | Comma-separated packages to exclude | |
| `-version` | Show version information | |

## Rules

### unwrapped-external-error

Errors from external packages must be wrapped with context:

```go
// Bad
file, err := os.Open("config.yml")
if err != nil {
    return err // Error: should be wrapped
}

// Good
file, err := os.Open("config.yml")
if err != nil {
    return fmt.Errorf("failed to open config: %w", err)
}
```

### double-wrap

Warns when an error is wrapped multiple times in the same package:

```go
// Bad
err := fmt.Errorf("step 1: %w", originalErr)
err = fmt.Errorf("step 2: %w", err) // Warning: already wrapped

// Good
err := fmt.Errorf("complete context: %w", originalErr)
```

### using-percent-v

Use `%w` instead of `%v` when wrapping errors:

```go
// Bad
return fmt.Errorf("operation failed: %v", err) // Error: use %w

// Good
return fmt.Errorf("operation failed: %w", err)
```

### missing-context

When `require-context` is enabled, errors must include descriptive context:

```go
// Bad
return fmt.Errorf("%w", err) // Warning: missing context

// Good
return fmt.Errorf("failed to process user data: %w", err)
```

### max-depth-exceeded

Warns when error wrapping chain becomes too deep:

```go
// Warning if depth > max-wrap-depth
err = fmt.Errorf("layer 1: %w", err)
err = fmt.Errorf("layer 2: %w", err)
// ... continues beyond max depth
```

## Sentinel Errors

The linter recognizes common sentinel errors that don't require wrapping:

- `io.EOF`, `io.ErrClosedPipe`, etc.
- `sql.ErrNoRows`, `sql.ErrTxDone`
- `context.Canceled`, `context.DeadlineExceeded`
- `os.ErrNotExist`, `os.ErrPermission`

## Custom Error Wrappers

The linter supports custom error wrapping functions:

```yaml
custom-wrappers:
  packages:
    - package: "github.com/mycompany/errors"
      functions: ["Wrap", "WrapWithContext"]
```

It also auto-detects types implementing the `Unwrap() error` method.

## Excluding Files and Packages

### Via Configuration

```yaml
exclude:
  files:
    - "*_test.go"
    - "*.pb.go"
  packages:
    - "generated"
```

### Via Comments

```go
//nolint:wrap-error
func SomeFunction() error {
    return externalErr // This line won't be flagged
}
```

## Output Formats

### Text (Default)

```
pkg/service/user.go:45:12: [ERROR][unwrapped-external-error] error from external package 'database/sql' should be wrapped
pkg/service/user.go:67:9: [WARN][double-wrap] error is already wrapped
```

### JSON

```json
{
  "issues": [
    {
      "file": "pkg/service/user.go",
      "line": 45,
      "column": 12,
      "rule": "unwrapped-external-error",
      "severity": "error",
      "message": "error from external package 'database/sql' should be wrapped"
    }
  ],
  "summary": {
    "total": 1,
    "errors": 1,
    "warnings": 0,
    "info": 0
  }
}
```

### HTML

Generates an interactive HTML report with sortable columns and syntax highlighting.

## Integration with CI/CD

### GitHub Actions

```yaml
- name: Run wrap-error-linter
  run: |
    go install github.com/ovargas/wrap-error-linter/cmd/wrap-error-linter@latest
    wrap-error-linter -mode fail -output json -output-path report.json ./...
```

### GitLab CI

```yaml
lint:
  script:
    - go install github.com/ovargas/wrap-error-linter/cmd/wrap-error-linter@latest
    - wrap-error-linter -mode fail ./...
```

## Development

### Building from Source

```bash
git clone https://github.com/ovargas/wrap-error-linter.git
cd wrap-error-linter
go build ./cmd/wrap-error-linter
```

### Running Tests

```bash
go test ./...
```

### Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see LICENSE file for details

## Support

For issues and feature requests, please use the [GitHub issue tracker](https://github.com/ovargas/wrap-error-linter/issues).