#!/bin/bash
set -ex

# Go to the root of the repository and build Caddy
pushd "$(dirname "$0")/../.." > /dev/null
go build -o caddy ./cmd/caddy
popd > /dev/null

# Go to the example directory
pushd "$(dirname "$0")" > /dev/null

# Run Caddy with the proxy config
../../caddy run --config caddy.config --adapter caddyfile

popd > /dev/null
