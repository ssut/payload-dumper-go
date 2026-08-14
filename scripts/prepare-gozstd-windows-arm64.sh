#!/usr/bin/env bash
set -euo pipefail

log() {
  local level="$1"
  shift
  printf 'ts=%s level=%s msg="%s" %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$level" "$1" "${2:-}"
}

die() {
  log error "$1" "${2:-}"
  exit 1
}

CC="${CC:-aarch64-w64-mingw32-gcc}"
AR="${AR:-aarch64-w64-mingw32-ar}"
RANLIB="${RANLIB:-aarch64-w64-mingw32-ranlib}"
SYSROOT="${WINDOWS_ARM64_SYSROOT:-/windows-arm64-buildroot/sys-root}"
OUT="$SYSROOT/lib/libzstd.a"

command -v "$CC" >/dev/null 2>&1 || die "missing C compiler" "cc=$CC"
command -v "$AR" >/dev/null 2>&1 || die "missing archiver" "ar=$AR"

if [ -f "$OUT" ]; then
  log info "libzstd already present" "path=$OUT"
  exit 0
fi

if [ -n "${ZSTD_SRC:-}" ]; then
  zstd_root="$ZSTD_SRC"
else
  command -v go >/dev/null 2>&1 || die "missing go toolchain and ZSTD_SRC" ""
  log info "downloading gozstd module for vendored zstd" "module=github.com/valyala/gozstd"
  go mod download github.com/valyala/gozstd
  zstd_root="$(go list -m -f '{{.Dir}}' github.com/valyala/gozstd)/zstd"
fi

[ -d "$zstd_root/lib" ] || die "zstd lib directory not found" "dir=$zstd_root/lib"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cp -aR "$zstd_root" "$work/zstd"

log info "building libzstd for windows/arm64" "cc=$CC ar=$AR src=$zstd_root"
make -C "$work/zstd/lib" \
  CC="$CC" \
  AR="$AR" \
  RANLIB="$RANLIB" \
  TARGET_SYSTEM=Windows \
  OS=Windows_NT \
  UNAME=Windows \
  UNAME_TARGET_SYSTEM=Windows \
  ZSTD_LEGACY_SUPPORT=0 \
  ZSTD_NO_ASM=1 \
  libzstd.a

[ -f "$work/zstd/lib/libzstd.a" ] || die "libzstd.a was not produced" "work=$work/zstd/lib"

mkdir -p "$SYSROOT/lib"
cp -f "$work/zstd/lib/libzstd.a" "$OUT"
[ -f "$OUT" ] || die "failed to install libzstd.a" "path=$OUT"
log info "installed libzstd" "path=$OUT"
