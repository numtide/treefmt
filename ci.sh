#!/usr/bin/env bash
set -euo pipefail

set -x

nix-shell --pure --run "cargo build"

nix-shell --pure --run "cargo test"

nix-build -A defaultNix.defaultPackage.x86_64-linux
