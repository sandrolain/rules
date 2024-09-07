#!/bin/bash

set -e

# Check if buf is installed
if ! command -v buf &> /dev/null
then
    echo "buf is not installed. Please install it first."
    echo "You can install it using: go install github.com/bufbuild/buf/cmd/buf@latest"
    exit 1
fi

# Update buf modules
echo "Updating buf modules..."
buf dep update

# Generate code
echo "Generating code..."
buf generate

echo "Proto build completed successfully!"