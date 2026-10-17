#!/usr/bin/env bash
# Cloud Agent repository bootstrap for arclint.
#
# Idempotent: installs the mise-pinned developer toolchain
# (golangci-lint, spectral, gitleaks, goreleaser, hk, zola), primes the
# Go module cache, and wires mise into interactive shells so `make check`
# and `mise run check` work out of the box.
set -euo pipefail

export PATH="$HOME/.local/bin:$PATH"

if ! command -v mise >/dev/null 2>&1; then
  curl -fsSL https://mise.run | sh
fi

MISE="$(command -v mise || echo "$HOME/.local/bin/mise")"

# Make the pinned tools available in the agent's interactive shells
# across reboots without re-running this script.
ACTIVATE_LINE='eval "$("'"$MISE"'" activate bash)"'
if ! grep -qF "$ACTIVATE_LINE" "$HOME/.bashrc" 2>/dev/null; then
  {
    echo ''
    echo '# arclint dev toolchain (mise)'
    echo "$ACTIVATE_LINE"
  } >>"$HOME/.bashrc"
fi

cd "$(dirname "$0")/.."

"$MISE" install

# Prime the module cache so the first build/test is fast and offline-safe.
"$MISE" exec -- go mod download
