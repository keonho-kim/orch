#!/bin/sh

set -eu

repo="${ORCH_INSTALL_REPO:-keonho-kim/orch}"
api_url="${ORCH_INSTALL_API_URL:-https://api.github.com/repos/$repo/releases/latest}"
download_base_url="${ORCH_INSTALL_DOWNLOAD_BASE_URL:-https://github.com/$repo/releases/download}"
bindir="${BINDIR:-}"
version="${VERSION:-}"

log() {
	printf '%s\n' "$*"
}

fail() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

download_to() {
	url="$1"
	output="$2"

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$output"
		return
	fi
	if command -v wget >/dev/null 2>&1; then
		wget -qO "$output" "$url"
		return
	fi

	fail "curl or wget is required"
}

download_text() {
	url="$1"

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url"
		return
	fi
	if command -v wget >/dev/null 2>&1; then
		wget -qO- "$url"
		return
	fi

	fail "curl or wget is required"
}

sha256_file() {
	file="$1"

	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file" | awk '{print $1}'
		return
	fi
	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$file" | awk '{print $1}'
		return
	fi

	fail "sha256sum or shasum is required"
}

install_file() {
	source="$1"
	target="$2"

	if command -v install >/dev/null 2>&1; then
		install -m 0755 "$source" "$target"
		return
	fi

	cp "$source" "$target"
	chmod 0755 "$target"
}

resolve_os() {
	case "$(uname -s)" in
		Darwin)
			printf 'darwin\n'
			;;
		Linux)
			printf 'linux\n'
			;;
		*)
			fail "unsupported operating system: $(uname -s)"
			;;
	esac
}

resolve_arch() {
	case "$(uname -m)" in
		x86_64|amd64)
			printf 'amd64\n'
			;;
		arm64|aarch64)
			printf 'arm64\n'
			;;
		*)
			fail "unsupported architecture: $(uname -m)"
			;;
	esac
}

resolve_bindir() {
	if [ -n "$bindir" ]; then
		printf '%s\n' "$bindir"
		return
	fi

	if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
		printf '/usr/local/bin\n'
		return
	fi

	printf '%s/.local/bin\n' "$HOME"
}

resolve_version_tag() {
	if [ -n "$version" ]; then
		case "$version" in
			v*)
				printf '%s\n' "$version"
				;;
			*)
				printf 'v%s\n' "$version"
				;;
		esac
		return
	fi

	response="$(download_text "$api_url")"
	tag="$(printf '%s' "$response" | tr -d '\n' | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p')"
	[ -n "$tag" ] || fail "failed to resolve latest release version"
	printf '%s\n' "$tag"
}

need_cmd tar
need_cmd find
need_cmd awk
need_cmd sed
need_cmd tr
need_cmd mktemp
need_cmd mkdir

os="$(resolve_os)"
arch="$(resolve_arch)"
bindir="$(resolve_bindir)"
tag="$(resolve_version_tag)"
version_core="${tag#v}"
archive="orch_${version_core}_${os}_${arch}.tar.gz"
archive_url="$download_base_url/$tag/$archive"
checksums_url="$download_base_url/$tag/checksums.txt"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM HUP

archive_path="$tmpdir/$archive"
checksums_path="$tmpdir/checksums.txt"
extract_dir="$tmpdir/extract"

mkdir -p "$extract_dir"
mkdir -p "$bindir"

log "Downloading $archive_url"
download_to "$archive_url" "$archive_path"
download_to "$checksums_url" "$checksums_path"

expected_sum="$(awk -v file="$archive" '$2 == file {print $1}' "$checksums_path")"
[ -n "$expected_sum" ] || fail "checksum for $archive not found"
actual_sum="$(sha256_file "$archive_path")"
[ "$expected_sum" = "$actual_sum" ] || fail "checksum mismatch for $archive"

tar -xzf "$archive_path" -C "$extract_dir"

orch_bin="$(find "$extract_dir" -type f -name orch | head -n 1)"
ot_bin="$(find "$extract_dir" -type f -name ot | head -n 1)"
[ -n "$orch_bin" ] || fail "orch binary not found in archive"
[ -n "$ot_bin" ] || fail "ot binary not found in archive"

install_file "$orch_bin" "$bindir/orch"
install_file "$ot_bin" "$bindir/ot"

log "Installed orch and ot to $bindir"
case ":${PATH:-}:" in
	*:"$bindir":*)
		;;
	*)
		log "Add $bindir to PATH to use orch from new shells."
		;;
esac
