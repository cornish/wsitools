# Changelog

All notable changes to wsi-tools will be documented here. The format is loosely based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] — 2026-05-08

opentile-go v0.14 alignment. Bumps the upstream dep from v0.12 to
v0.14, claims the new capabilities that bump unlocks (novel-codec
recognition + wsi-tools ImageDescription parsing on re-read), and
migrates the streaming transcode hot path to opentile-go's
allocation-free `TileInto` API with a per-level `sync.Pool` of
tile-sized buffers.

### Added

- **`internal/source.Compression`** gains three new values —
  `CompressionWebP`, `CompressionJPEGXL`, `CompressionHTJ2K` —
  matching opentile-go v0.14's new enum values. AVIF was already
  mapped via the v0.8 `CompressionAVIF` constant.
- **`pipeline.Tile.Release func()`** — optional buffer-pool callback,
  invoked by the consumer between decode and encode. Nil-safe; the
  pipeline package itself stays opaque to the field.
- **opentile-go round-trip integration test for the 4 novel codecs**
  — replaces the prior `tiffinfo` shell-out with assertions on
  `Format()`, `Compression()`, `TileSize()`, and
  `Metadata.AcquisitionDateTime`.

### Changed

- **opentile-go bumped from v0.12 → v0.14.** Both v0.13 (additive
  splice-prefix family on `Level`) and v0.14 (additive Compression
  values + wsi-tools ImageDescription parser) are non-breaking.
- **`internal/source.Level` interface** — `Tile()` removed; replaced
  by `TileMaxSize()` and `TileInto(x, y int, dst []byte) (int, error)`.
  `internal/` is private API; no external callers affected. The
  transcode producer now uses a per-level `sync.Pool`; the downsample
  source loop hoists a single tile-sized buffer above the loop.
- **`cmd/wsi-tools/version.go::Version`** bumped to `0.3.0-dev`. The
  literal `"wsi-tools/0.2.0-dev"` strings in the transcode provenance
  builder and the `WithToolsVersion` writer option are now derived
  from `Version`, not hardcoded.

### Not adopted (intentional)

- `opentile.Level.TilePrefix` / `TileBodyInto` / `SpliceJPEGTile` are
  bandwidth-deduplication helpers for client-server byte-passthrough
  scenarios. wsi-tools' transcode pipeline fully decodes every tile,
  so the splice family offers no benefit. The decision is reversible
  if a future feature (e.g., a streaming HTTP tile server) needs it.

## [0.2.0] — 2026-05-08

The transcode milestone. Adds `wsi-tools transcode` with 4 new codec wrappers, expands source format support to 6 sane TIFF dialects, ships a streaming pyramid pipeline that lifts the v0.1 memory ceiling, and bundles a fix for v0.1 downsample's associated-image IFD-ordering bug.

### Added

- **`wsi-tools transcode`** — re-encode the pyramid tiles in a different compression codec while preserving source tile geometry and metadata. Associated images (label, macro, thumbnail, overview) pass through verbatim.
  - **Codec targets**: `jpegxl` (libjxl, JPEG-XL codestream per tile), `avif` (libavif), `webp` (libwebp), `htj2k` (OpenJPH HTJ2K codestream). Plus the v0.1 `jpeg` codec available as a transcode target for re-encoding at a different quality. **Total: 5 `--codec` values accepted at v0.2.0.**
  - **Source formats**: SVS, Philips-TIFF, OME-TIFF (tiled SubIFD path), BIF (Ventana), IFE (Iris), generic-TIFF. NDPI, OME-OneFrame, and Leica SCN error cleanly with `ErrUnsupportedFormat`.
  - **Streaming end-to-end**: per-tile decode → encode → write, no L0 raster materialisation. Memory ceiling drops three orders of magnitude vs. v0.1 downsample (≈ workers × tile_bytes × 2, independent of slide size).
  - **Output container**: SVS-shaped when source is SVS AND codec is `jpeg` (Aperio convention); generic pyramidal TIFF otherwise.
  - **Per-codec quality knobs**: single `--quality 1..100` mapped per-codec; `--codec-opt key=val` for codec-specific tuning (`jxl.distance`, `jxl.effort`, `avif.speed`, `webp.lossless`, etc.).
  - **Per-codec build tags**: `-tags nojxl noavif nowebp nohtj2k` produce slim binaries that skip selected codecs.
- **`internal/source`** — adapter package between the CLI and opentile-go. Encapsulates the sanity gate (NDPI / OME-OneFrame / Leica SCN rejection) and exposes a unified streaming-friendly tile API. The `ReadSourceImageDescription` helper (TIFF tag-270 reader) was promoted from `cmd/wsi-tools/downsample.go` so transcode + downsample share the implementation.
- **`internal/wsiwriter`** — extended with self-describing TIFF tags:
  - `WSIImageType` (private tag 65080, ASCII): one of `pyramid`, `label`, `macro`, `overview`, `thumbnail`, `probability`, `map`, `associated`. Aligns with DICOM-WSI's `ImageType` vocabulary.
  - `WSILevelIndex` (65081, LONG), `WSILevelCount` (65082, LONG): emitted on pyramid IFDs.
  - `WSISourceFormat` (65083, ASCII), `WSIToolsVersion` (65084, ASCII): emitted on L0.
  - Standard TIFF metadata tags now populated from opentile-go's cross-format metadata: 271 Make, 272 Model, 305 Software, 306 DateTime.
  - Documented in `docs/tiff-tags.md` (renamed from `docs/compression-tags.md`).
- **Integration test sweep** (`tests/integration/transcode_test.go`):
  - Per-codec sweep (jpeg + 4 v0.2 codecs) with structural validation via `tiffinfo` for codecs opentile-go doesn't yet decode.
  - Per-source-format sweep across all 6 sane source formats. Includes NDPI + Leica SCN rejection cases.
  - 4.8 GB BigTIFF fixture re-included (v0.1 excluded it for memory reasons; streaming makes it tractable).
- **CI**: macOS workflow installs `jpeg-xl libavif webp openjph` in addition to v0.1's deps. Windows workflow adds `libjxl libavif libwebp` via msys2; OpenJPH is not packaged for msys2 yet, so the Windows build uses `-tags nohtj2k`.

### Fixed

- **`wsi-tools downsample` IFD-ordering bug**: v0.1 wrote `L0, L1, …, LN, thumbnail, label, macro` (pyramid first, all associated at end). opentile-go's SVS classifier (`formats/svs/series.classifyPages`) takes the LAST 2 trailing pages as label/macro, so thumbnail was being misclassified as label and the real label getting reclassified as macro on re-read. v0.2 corrects the ordering to `L0, [thumbnail], L1, …, LN, label, macro` matching Aperio's convention. Verified by `TestDownsample_AssociatedKindRoundTrip`.

### Changed

- **opentile-go bumped from v0.10 → v0.12**. v0.12 renames `FormatPhilips` → `FormatPhilipsTIFF` (`"philips-tiff"`) and `FormatOME` → `FormatOMETIFF` (`"ome-tiff"`), and v0.11 added `FormatLeicaSCN`. wsi-tools now references `opentile.Format*` constants rather than literal strings, insulating future renames.
- `cmd/wsi-tools/downsample.go`'s local `readSourceImageDescription` helper promoted to `internal/source.ReadSourceImageDescription`.

### Deferred to v0.2.x or later

- **`jpegli` codec**: was originally part of v0.2.0 but Homebrew's `jpeg-xl 0.11.2` bottle ships libjxl without `libjpegli` (upstream disables it to avoid libjpeg symbol-conflicts). Defer to v0.2.1+ once we either get an upstream re-enable or stand up a build-from-source path.
- **HEIF, JPEG-LS, JPEG-XR, Basis Universal codecs**: queued for v0.2.x.
- **`jpeg2000` as a transcode target**: decoder is shipped; encoder wrapper is queued for v0.2.x.
- **Streaming retrofit for `downsample`**: v0.2.0 ships streaming for transcode only; downsample still materialises the full L0 raster. v0.2.x.
- **Leica SCN source support**: SCN's multi-image + multi-channel structure requires per-`Image` and per-channel pipeline plumbing not in v0.2.0 scope.
- **Visual-fidelity tests via mini decoders** (read raw tile bytes from opentile-go, decode via the matching codec library): v0.2.x follow-up to validate JPEG-XL / AVIF / WebP / HTJ2K outputs without depending on opentile-go to grow decoders for those compression IDs.
- ~~**opentile-go decoders for JXL / AVIF / WebP / HTJ2K compression tags**~~: **landed in v0.3.0** via opentile-go v0.14's new `Compression` enum values + generic-TIFF tag mappings. opentile-go does not decode the tile bytes (byte-passthrough contract — consumers bring their own codec libraries) but recognises the compression tags and parses the wsi-tools `ImageDescription`.

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
