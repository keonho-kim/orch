#!/usr/bin/env bash
set -euo pipefail

path=""
from_stdin="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --path)
      path="${2:-}"
      shift 2
      ;;
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

if [[ -z "$path" ]]; then
  echo "--path is required" >&2
  exit 1
fi

if [[ "$from_stdin" != "true" ]]; then
  echo "--from-stdin is required" >&2
  exit 1
fi

target="$OT_WORKSPACE_ROOT/$path"
if [[ -d "$target" ]]; then
  echo "ot write requires a file path, not a directory" >&2
  exit 1
fi
mkdir -p "$(dirname "$target")"
cat > "$target"
echo "wrote $path"
