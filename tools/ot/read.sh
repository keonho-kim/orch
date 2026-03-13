#!/usr/bin/env bash
set -euo pipefail

path=""
start=""
end=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --path)
      path="${2:-}"
      shift 2
      ;;
    --start)
      start="${2:-}"
      shift 2
      ;;
    --end)
      end="${2:-}"
      shift 2
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

target="$OT_WORKSPACE_ROOT/$path"
if [[ ! -e "$target" ]]; then
  echo "path not found: $path" >&2
  exit 1
fi

if [[ -d "$target" ]]; then
  if [[ -n "$start" || -n "$end" ]]; then
    echo "ot read line ranges are only supported for files" >&2
    exit 1
  fi
  find "$target" -mindepth 1 -maxdepth 1 -print | sed "s#^$OT_WORKSPACE_ROOT/##"
  exit 0
fi

if [[ ! -f "$target" ]]; then
  echo "unsupported read target: $path" >&2
  exit 1
fi

if [[ -n "$start" || -n "$end" ]]; then
  from="${start:-1}"
  to="${end:-999999}"
  sed -n "${from},${to}p" "$target"
  exit 0
fi

cat "$target"
