# Changelog

## [Unreleased]

### Added

- dm-verity FEC (forward error correction) generation for incremental (delta) payloads. Delta payloads omit the Reed-Solomon parity for verity-protected partitions and expect the device to compute it while installing; payload-dumper-go now computes it too, so partitions such as `system`, `product`, `vendor` and `odm` extract bit-exact and pass their own sha256 check. Previously these failed with "requires FEC (forward error correction) data reconstruction, which is not supported yet" ([#69](https://github.com/ssut/payload-dumper-go/issues/69)).
- `-no-fec` to skip FEC generation. The FEC region is left unwritten and the image will not match its expected sha256, so it must not be flashed; it is only useful for inspecting an image's contents.
- `ProgressEvent.Phase`, `PhaseCompleted` and `PhaseTotal`, reporting progress of the verity phase that runs after the install operations. `ProgressFunc` implementations must be safe for concurrent use.

### Changed

- The dm-verity hash tree and FEC parity are now regenerated whenever the payload declares the corresponding extents, instead of only when a partition consumes source images. A delta partition whose operations are all full-replace previously skipped hash tree generation and failed output verification.
- Extraction is cancellable during the verity phase. Previously `ctx` was only checked between install operations, so Ctrl-C did not interrupt hash tree or FEC work.

## [2.0.2] - 2026-08-20

### Changed

- Built with Go 1.27 (`go.mod` go directive 1.25.0 -> 1.27.0). CI and release builds pick up `go1.27.0` automatically through `GOTOOLCHAIN=auto`.

### Note

- macOS builds now require macOS 13 Ventura or later, following Go 1.27 dropping support for earlier macOS versions.

## [2.0.1] - 2026-08-19

### Added

- windows/arm64 release artifact, cross-compiled with llvm-mingw against a dedicated liblzma/libzstd sysroot.

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
