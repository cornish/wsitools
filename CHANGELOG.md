# Changelog

All notable changes to wsi-tools will be documented here. The format is loosely based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-05-07

First release. Ships the `downsample` subcommand end-to-end on Aperio SVS sources.

### Added

- **`wsi-tools downsample`** — produce a lower-magnification copy of an Aperio SVS by an integer power-of-2 factor (default 2 = 40x → 20x). Regenerates the entire pyramid from the new L0; passes through associated images (label, macro, thumbnail, overview) verbatim.
  - Source codecs: JPEG (libjpeg-turbo, with 1/N in-decode fast scale) and JPEG 2000 (OpenJPEG, full decode + 2×2 area average chain).
  - Output: Aperio-shaped SVS (or BigTIFF when predicted output > 2 GiB).
  - Flags: `--factor`, `--target-mag`, `--quality`, `--jobs`, `--bigtiff` (auto), `--force`, `--quiet`, `--verbose`, `--log-level`, `--log-format`.
- **`wsi-tools doctor`** — list registered codecs and required cgo libraries.
- **`wsi-tools version`** — print version + Go runtime info.
- **`internal/wsiwriter`** — pure-Go TIFF / BigTIFF / Aperio-SVS writer with stripped + tiled IFDs, atomic close, abbreviated-JPEG tile mode + per-level `JPEGTables`, and `ImageDescription` mutation for downsample.
- **`internal/codec/jpeg`** — libjpeg-turbo encoder writing raw-RGB-storage JPEGs with the Aperio Adobe APP14 marker (matches what real Aperio scanners emit).
- **`internal/decoder/{jpeg,jpeg2000}`** — libjpeg-turbo + OpenJPEG decoders, with libjpeg-turbo's 1/N fast-scale in-decode path for the JPEG case.
- **`internal/resample`** — 2×2 area-average resampler (pure Go); Lanczos plumbed as a stub returning `ErrNotImplemented` until v0.2.
- **`internal/pipeline`** — worker-pool decode → process → encode plumbing with cancellation via `context.WithCancelCause`, atomic on-disk semantics, and SIGINT/SIGTERM handling at the CLI layer.
- Progress bar (`vbauerster/mpb`) + structured logging (`log/slog`, text or JSON).
- Integration test suite (`-tags integration`) gated by `WSI_TOOLS_TESTDIR`, sweeping the standard opentile-go fixture pool.
- CI: macOS (build + test), Windows (build only). Linux untested but expected to work.

### Known limitations

- v0.1 holds the full L0 raster in memory during pyramid build (≈4.5 GiB at 20x sources, ≈18 GiB at 40x sources). Streaming pyramid build is the headline v0.2 deliverable.
- Lanczos resampler is stubbed; `--factor` rejects non-power-of-2 values at the CLI layer until v0.2.
- libjpeg's default `error_exit` calls `exit(1)` on any libjpeg error; production-grade error recovery requires installing a custom `longjmp`-based handler. Acceptable for v0.1 since input validation happens up front.
- No transcode tool yet — that's the v0.2 milestone, with 11 codec targets (jpegli, JPEG-XL, AVIF, WebP, HEIF, JPEG-LS, JPEG-XR, HTJ2K, Basis Universal, plus jpeg/jpeg2000 baselines).
- SVS sources only. NDPI, Philips, OME-TIFF, BIF, IFE deferred to v0.2+.

### Notes

This is a from-scratch v0 release; no prior version history exists. The implementation plan and design spec live at `docs/superpowers/specs/2026-05-06-wsi-tools-v01-design.md` and `docs/superpowers/plans/2026-05-06-wsi-tools-v01-foundation-and-downsample.md` for posterity.
