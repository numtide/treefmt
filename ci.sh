#!/usr/bin/env bash
# nix-shell loads the devshell making rust and all the formatters available.
set -exuo pipefail

# Quick sanity check
cargo test
cargo test -- --ignored --test-threads=1

# Check that no code needs reformatting. Acts as a minimal integration test.
cargo run -- --fail-on-change

# Build the nix package
nix-build --no-out-link
