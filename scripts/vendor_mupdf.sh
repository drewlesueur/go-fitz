#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/vendor_mupdf.sh [options]

Options:
  --mupdf-root <path>   Path to MuPDF repo root (default: ../mupdf)
  --build <name>        Build type to copy libs from (default: release)
  --no-headers          Skip copying headers
  --no-libs             Skip copying libs
  -h, --help            Show this help

This script updates the vendored MuPDF headers and the current OS/arch
static libs used by go-fitz. It does not build MuPDF for you.

Examples:
  scripts/vendor_mupdf.sh --mupdf-root /home/chirag/mupdf
  scripts/vendor_mupdf.sh --build release
USAGE
}

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mupdf_root="$root_dir/../mupdf"
build="release"
copy_headers="yes"
copy_libs="yes"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mupdf-root)
      mupdf_root="$2"
      shift 2
      ;;
    --build)
      build="$2"
      shift 2
      ;;
    --no-headers)
      copy_headers="no"
      shift
      ;;
    --no-libs)
      copy_libs="no"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -d "$mupdf_root" ]]; then
  echo "MuPDF root not found: $mupdf_root" >&2
  exit 1
fi

if [[ "$copy_headers" == "yes" ]]; then
  if [[ ! -d "$mupdf_root/include/mupdf" ]]; then
    echo "MuPDF headers not found: $mupdf_root/include/mupdf" >&2
    exit 1
  fi
  rm -rf "$root_dir/include/mupdf"
  cp -a "$mupdf_root/include/mupdf" "$root_dir/include/"

  # Restore vendor stubs required by go-fitz
  cat >"$root_dir/include/mupdf/vendor.go" <<'EOF2'
//go:build required

package vendor
EOF2
  cat >"$root_dir/include/mupdf/fitz/vendor.go" <<'EOF3'
//go:build required

package vendor
EOF3
fi

if [[ "$copy_libs" == "yes" ]]; then
  libdir="$mupdf_root/build/$build"
  if [[ ! -d "$libdir" ]]; then
    echo "MuPDF build dir not found: $libdir" >&2
    exit 1
  fi
  if [[ ! -f "$libdir/libmupdf.a" ]] || [[ ! -f "$libdir/libmupdf-third.a" ]]; then
    echo "MuPDF libs not found in: $libdir" >&2
    echo "Expected libmupdf.a and libmupdf-third.a" >&2
    exit 1
  fi

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch_raw="$(uname -m)"
  case "$arch_raw" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "Unsupported arch: $arch_raw" >&2
      exit 1
      ;;
  esac

  case "$os" in
    linux|darwin)
      ;;
    *)
      echo "Unsupported OS: $os" >&2
      exit 1
      ;;
  esac

  cp -a "$libdir/libmupdf.a" "$root_dir/libs/libmupdf_${os}_${arch}.a"
  cp -a "$libdir/libmupdf-third.a" "$root_dir/libs/libmupdfthird_${os}_${arch}.a"
fi

echo "MuPDF vendoring complete."
