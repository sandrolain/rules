#!/bin/bash

set -e

# Run tests with coverage
go test ./... -coverprofile=coverage.out

# Display coverage report in the terminal
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

echo "Coverage report generated: coverage.html"

# Open the coverage report in the default browser
case "$(uname -s)" in
    Linux*)     xdg-open coverage.html ;;
    Darwin*)    open coverage.html ;;
    CYGWIN*|MINGW32*|MSYS*|MINGW*) start coverage.html ;;
    *)          echo "Unsupported operating system. Please open coverage.html manually." ;;
esac