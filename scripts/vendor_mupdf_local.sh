#!/usr/bin/env bash
set -euo pipefail

MUPDF_ROOT="/home/ubuntu/mupdf"
RUN_FULL_VENDOR=0

usage() {
  cat >&2 <<USAGE
Usage: $0 [--mupdf-root PATH] [--full]

  --mupdf-root PATH   Path to MuPDF repo (default: /home/ubuntu/mupdf)
  --full              After local OS/arch copy, run vendor_mupdf.sh
USAGE
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mupdf-root)
      MUPDF_ROOT="$2"
      shift 2
      ;;
    --full)
      RUN_FULL_VENDOR=1
      shift
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage
      ;;
  esac
 done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_FITZ_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported arch: $ARCH_RAW" >&2
    exit 1
    ;;
 esac

case "$OS" in
  linux|darwin)
    ;; 
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
 esac

LIB_MUPDF_SRC="$MUPDF_ROOT/build/release/libmupdf.a"
LIB_THIRD_SRC="$MUPDF_ROOT/build/release/libmupdf-third.a"
LIB_MUPDF_DST="$GO_FITZ_ROOT/libs/libmupdf_${OS}_${ARCH}.a"
LIB_THIRD_DST="$GO_FITZ_ROOT/libs/libmupdfthird_${OS}_${ARCH}.a"

if [[ ! -d "$MUPDF_ROOT" ]]; then
  echo "MuPDF root not found: $MUPDF_ROOT" >&2
  exit 1
fi

if [[ ! -d "$GO_FITZ_ROOT/libs" ]]; then
  echo "go-fitz libs dir not found: $GO_FITZ_ROOT/libs" >&2
  exit 1
fi

echo "Building MuPDF in $MUPDF_ROOT (build=release)"
make -C "$MUPDF_ROOT" clean
make -C "$MUPDF_ROOT" build=release

if [[ ! -f "$LIB_MUPDF_SRC" || ! -f "$LIB_THIRD_SRC" ]]; then
  echo "Expected MuPDF libs not found in $MUPDF_ROOT/build/release" >&2
  exit 1
fi

echo "Vendoring libs to go-fitz ($OS/$ARCH)"
cp "$LIB_MUPDF_SRC" "$LIB_MUPDF_DST"
cp "$LIB_THIRD_SRC" "$LIB_THIRD_DST"

echo "Done local copy:"
ls -l "$LIB_MUPDF_DST" "$LIB_THIRD_DST"

if [[ "$RUN_FULL_VENDOR" -eq 1 ]]; then
  echo "Running full vendor script..."
  "$SCRIPT_DIR/vendor_mupdf.sh" --mupdf-root "$MUPDF_ROOT" --build release
fi
