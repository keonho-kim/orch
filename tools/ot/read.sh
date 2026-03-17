#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=/dev/null
. "$script_dir/common.sh"

target=""
scope=""
display=""
workspace_root=""
start=""
end=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      target="${2:-}"
      shift 2
      ;;
    --scope)
      scope="${2:-}"
      shift 2
      ;;
    --display)
      display="${2:-}"
      shift 2
      ;;
    --workspace-root)
      workspace_root="${2:-}"
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
      ot_die "unknown arg: $1"
      ;;
  esac
done

ot_require_common_args "$target" "$scope" "$display" "$workspace_root"
ot_reject_hidden_external_target "$scope" "$target"

if [[ ! -e "$target" ]]; then
  ot_die "path not found: $target"
fi

if [[ -d "$target" ]]; then
  if [[ -n "$start" || -n "$end" ]]; then
    ot_die "ot read line ranges are only supported for files"
  fi

  if [[ "$scope" == "inside" ]]; then
    while IFS= read -r entry; do
      ot_display_path "$entry" "$display" "$workspace_root"
    done < <(find "$target" -mindepth 1 -maxdepth 1 -print)
  else
    while IFS= read -r entry; do
      ot_display_path "$entry" "$display" "$workspace_root"
    done < <(find "$target" -mindepth 1 -maxdepth 1 ! -name '.*' -print)
  fi
  exit 0
fi

if [[ ! -f "$target" ]]; then
  ot_die "unsupported read target: $target"
fi

if [[ -n "$start" || -n "$end" ]]; then
  from="${start:-1}"
  to="${end:-999999}"
  sed -n "${from},${to}p" "$target"
  exit 0
fi

cat "$target"
