#!/usr/bin/env bash
set -euo pipefail

if [[ $# -eq 0 ]]; then
  echo "ot exec requires a command" >&2
  exit 1
fi

"$@"
