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

## Working on the docs site

The documentation site lives on an orphan branch `docs-site` (Docusaurus machinery only — no markdown content, which is pulled from `main` at build time). The deploy workflow on `main` (`.github/workflows/docs.yml`) handles the build.

### One-time setup

```shell
git fetch origin docs-site
git worktree add website docs-site
cd website
npm install
```

`/website/` is gitignored on `main`, so the worktree doesn't pollute the index.

### Editing doc content (`docs/*.md`)

Edit on `main` as usual. A push to `main` that touches `docs/**`, `examples/README.md`, `*.go` (non-test), `README.md`, or `PERFORMANCE.md` triggers a rebuild.

To preview locally before pushing:

```shell
cd website
# Copy current main-branch content into the worktree's docs/ folder:
cp ../docs/*.md docs/
mkdir -p docs/about && cp ../PERFORMANCE.md docs/about/performance.md
npm run start
```

Open <http://localhost:3000/structpages/>.

### Editing site machinery (theme, sidebar, config)

Work in the `website/` worktree on the `docs-site` branch. Commit and push there:

```shell
cd website
git add .
git commit -m "chore(docs-site): ..."
git push origin docs-site
```

Then trigger a deploy from the Actions tab (`Docs Site` workflow → `Run workflow` on `main`) since pushes to `docs-site` don't auto-fire the workflow (it's defined on `main`).

## Questions or Issues?

Feel free to open an issue on GitHub if you have questions or run into problems.
