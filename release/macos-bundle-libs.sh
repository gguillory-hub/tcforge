#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <dependency-dir>" >&2
  exit 2
fi

deps_dir="$(cd "$1" && pwd)"
lib_dir="$deps_dir/lib"
mkdir -p "$lib_dir"

is_bundle_dependency() {
  [[ "$1" == /opt/homebrew/* || "$1" == /usr/local/opt/* || "$1" == /usr/local/Cellar/* ]]
}

copy_and_patch() {
  local file="$1"
  local mode="$2"
  local deps dep base dest replacement

  chmod u+w "$file" || true
  deps="$(otool -L "$file" | awk 'NR > 1 {print $1}')"

  while IFS= read -r dep; do
    [[ -z "$dep" ]] && continue
    if ! is_bundle_dependency "$dep"; then
      continue
    fi

    base="$(basename "$dep")"
    dest="$lib_dir/$base"
    if [[ "$mode" == "exe" ]]; then
      replacement="@executable_path/lib/$base"
    else
      replacement="@loader_path/$base"
    fi

    if [[ ! -f "$dest" ]]; then
      cp "$dep" "$dest"
      chmod u+w "$dest" || true
      install_name_tool -id "@loader_path/$base" "$dest" || true
      copy_and_patch "$dest" "lib"
    fi

    install_name_tool -change "$dep" "$replacement" "$file" || true
  done <<< "$deps"
}

for tool in ffmpeg ffprobe ltcdump; do
  if [[ -f "$deps_dir/$tool" ]]; then
    copy_and_patch "$deps_dir/$tool" "exe"
  fi
done
