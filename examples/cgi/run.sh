#!/bin/bash
set -e

# Go to the root of the repository
cd "$(dirname "$0")/../.."

# Build Caddy with the local version of the cgi module
xcaddy build --with github.com/aksdb/caddy-cgi/v2=.

# Go back to the example directory
cd examples/cgi

# Ensure the script is executable
chmod +x hello.sh

# Run Caddy with the example configuration
../../caddy run --config caddy.config --adapter caddyfile
