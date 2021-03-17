#!/nix/store/cwnwyy82wrq53820z6yg7869z8dl5s7g-bash-4.4-p23/bin/bash
# Build and serve the docs for local development
set -euo pipefail
webfsd -d -r "$(nix-build "$(dirname "$0")/.." -A docs)"
