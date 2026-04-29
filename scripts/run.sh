#!/usr/bin/env bash
# Convenience launcher for the mtgsquad-server.
# Run from the mtgsquad/ root.
set -euo pipefail

cd "$(dirname "$0")/.."

go run ./cmd/mtgsquad-server "$@"
