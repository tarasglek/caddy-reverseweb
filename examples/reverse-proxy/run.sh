#!/bin/bash
set -ex

# Change to the directory of this script
cd "$(dirname "$0")"

# Call the central run script
../../cmd/caddy/run.sh caddy.config
