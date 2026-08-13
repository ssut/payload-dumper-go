# Changelog

## [2.0.0] - 2026-08-14

### Added

- Incremental (delta) OTA support: `SOURCE_COPY`, `MOVE`, `SOURCE_BSDIFF`, `BSDIFF`, `BROTLI_BSDIFF` applied on top of base images (`-old <dir>`), with a pure Go bspatch (BSDIFF40/BSDF2).
- Virtual A/B fall-through (unmodified blocks carried over from base) and dm-verity hash tree computation, producing bit-exact images.
- sha256 verification of operation data, source images, and final images (`-no-verify` to skip).
- Go library package: `github.com/ssut/payload-dumper-go/payload`.
- `-q`/`-quiet` (#45) and `-m`/`-machine-readable` (#42) flags.
- payload.bin stored in an OTA zip is read in place, without a temp copy (#40, #61).

### Fixed

- Failures no longer exit with code 0: every error aborts with a non-zero exit code (#65).
- Operations with multiple destination extents write all extents, not just the first.

### Changed

- Codebase restructured into `payload/`, `cmd/` and `internal/` packages (rework of #57).
- Progress bars render on stderr; stdout carries only data output.
- Default worker count is the number of CPUs.
