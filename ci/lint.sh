#!/bin/bash
# CI entrypoint for formatting and linting checks.

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

export GOCACHE=$(mktemp -d /tmp/gocache.XXXXXX)
export GOMODCACHE=$(mktemp -d /tmp/gomodcache.XXXXXX)
export GOLANGCI_LINT_CACHE=$(mktemp -d /tmp/golangci-lint-cache.XXXXXX)
export GOFLAGS=-mod=mod

make deps
make fmt-check
make lint
