#!/usr/bin/env bash
set -euo pipefail

from_stdin="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from-stdin)
      from_stdin="true"
      shift
      ;;
    *)
      echo "unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [[ "$from_stdin" != "true" ]]; then
  echo "--from-stdin is required" >&2
  exit 1
fi

patch_bin="${OT_PATCH_BIN:-patch}"
"$patch_bin" -p0 -u
