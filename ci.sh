#!/usr/bin/env nix-shell
#!nix-shell -i bash
# nix-shell loads the devshell making rust and all the formatters available.
# shellcheck shell=bash
set -euo pipefail

set -x

# Quick sanity check
cargo test

# Check that no code needs reformatting. Acts as a minimal integration test.
cargo run -- --fail-on-change

# Build the nix package
nix-build -A defaultNix.defaultPackage.x86_64-linux
