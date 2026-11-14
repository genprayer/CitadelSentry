#!/usr/bin/env bash
set -euo pipefail

# Usage: ./scripts/release.sh patch|minor|major|<version>
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "${SCRIPT_DIR}/release.py" "$@"
