#!/bin/bash
# Script to set up git hooks for the project

echo "Setting up git hooks..."
git config core.hooksPath .githooks
echo "Git hooks configured to use .githooks directory"

# Make sure hooks are executable
chmod +x .githooks/*

# Install required tools
echo "Checking for required tools..."

# Check for goimports
if ! command -v goimports &> /dev/null; then
    echo "Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
fi

# Check for golangci-lint
if ! command -v golangci-lint &> /dev/null; then
    echo "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

# Check for staticcheck
if ! command -v staticcheck &> /dev/null; then
    echo "Installing staticcheck..."
    go install honnef.co/go/tools/cmd/staticcheck@latest
fi

echo "Setup complete! Pre-commit hooks will run automatically before each commit."
echo "To bypass hooks temporarily, use: git commit --no-verify"