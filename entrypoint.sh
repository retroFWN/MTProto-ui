#!/bin/sh
set -e

# Pre-build telemt (Rust) proxy image if it doesn't exist yet.
# This runs inside the panel container which has docker-cli + host socket mounted.
if ! docker images -q telemt-local 2>/dev/null | grep -q .; then
    echo "==> telemt-local image not found, building from source (this may take several minutes)..."
    if docker build -t telemt-local https://github.com/telemt/telemt.git; then
        echo "==> telemt-local image built successfully."
    else
        echo "==> WARNING: Failed to build telemt-local. Rust backend will not be available."
    fi
else
    echo "==> telemt-local image already exists, skipping build."
fi

exec ./mtproxy-panel "$@"
