# Contributing to StructPages

Thank you for your interest in contributing to StructPages!

## Development Setup

### Prerequisites

- Go 1.24.3 or later
- [Templ CLI](https://templ.guide/) for working with examples

### Setting Up Pre-commit Hooks

To ensure code quality and consistency, set up pre-commit hooks for automatic code formatting and linting:

```shell
./scripts/setup-hooks.sh
```

This will configure git to run `goimports`, `gofmt`, and `golangci-lint` before each commit.

### Running Tests

```shell
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -run TestName ./...
```

### Working with Examples

```shell
# Navigate to an example
cd examples/simple  # or examples/htmx or examples/todo

# Install dependencies
go mod download

# Generate Go code from Templ files (required before running)
templ generate -include-version=false

# Run the example server (typically on :8080)
go run main.go

# Watch mode for Templ files during development
templ generate --watch
```

## Code Guidelines

- Follow standard Go conventions and idioms
- Write tests for new functionality
- Update documentation when adding features
- Keep commits focused and atomic
- Write clear commit messages

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and ensure they pass
5. Commit your changes (pre-commit hooks will run automatically)
6. Push to your branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Questions or Issues?

Feel free to open an issue on GitHub if you have questions or run into problems.
