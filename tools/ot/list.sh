#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=/dev/null
. "$script_dir/common.sh"

target=""
scope=""
display=""
workspace_root=""

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

print_long_entry() {
  local entry="$1"
  local shown
  local line

  shown="$(ot_display_path "$entry" "$display" "$workspace_root")"
  line="$(LC_ALL=C ls -ld "$entry")"
  ot_rewrite_ls_line "$line" "$entry" "$shown"
}

if [[ -d "$target" ]]; then
  if [[ "$scope" == "inside" ]]; then
    while IFS= read -r entry; do
      print_long_entry "$entry"
    done < <(find "$target" -mindepth 1 -maxdepth 1 -print)
  else
    while IFS= read -r entry; do
      print_long_entry "$entry"
    done < <(find "$target" -mindepth 1 -maxdepth 1 ! -name '.*' -print)
  fi
  exit 0
fi

print_long_entry "$target"
