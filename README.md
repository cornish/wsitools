# wsi-tools

[![CI](https://github.com/cornish/wsi-tools/actions/workflows/ci.yml/badge.svg)](https://github.com/cornish/wsi-tools/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/cornish/wsi-tools.svg)](https://pkg.go.dev/github.com/cornish/wsi-tools)

A Swiss-army knife of utilities for whole-slide imaging (WSI) files used in digital pathology.

See [`CHANGELOG.md`](./CHANGELOG.md) for release notes.

## v0.4 — what's here

**Write-side**

- `wsi-tools downsample` — downsample a WSI by a power-of-2 factor (e.g. 40x → 20x).
  Regenerates the full pyramid from the new base. Passes through associated images
  (label, macro, thumbnail, overview) verbatim. SVS-only.
- `wsi-tools transcode` — re-encode the pyramid in a different codec while preserving
  source tile geometry and associated images. Streaming (no L0 raster materialisation).
  Five codec targets: `jpeg`, `jpegxl`, `avif`, `webp`, `htj2k`. Six source formats:
  SVS, Philips-TIFF, OME-TIFF (tiled), BIF, IFE, generic-TIFF. NDPI, OME-OneFrame,
  and Leica SCN error cleanly with `ErrUnsupportedFormat`.

**Read-side (v0.4)**

- `wsi-tools info` — slide summary: format, levels (dimensions + tile size + compression),
  associated images, scanner metadata. Text or `--json`. Analog of `openslide-show-properties`.
- `wsi-tools dump-ifds` — format-aware per-IFD layout dump. Annotates each IFD with its
  classification (pyramid L0/L1/…/label/macro/thumbnail/overview/probability/map) and
  reports wsi-tools private tags (65080–65084). Slim tiffinfo analog.
- `wsi-tools extract --kind <k> -o <path>` — save an associated image
  (label/macro/thumbnail/overview) as PNG (default) or JPEG. JPEG output is
  byte-pass-through when the source is already JPEG.
- `wsi-tools hash` — content hash. `--mode file` (default, `sha256sum`-equivalent) or
  `--mode pixel` (L0 RGB tiles in raster order, stable across re-encode).

**Diagnostics**

- `wsi-tools doctor` — report installed codec libraries.
- `wsi-tools version` — print version + Go runtime info.

## Roadmap

See [`docs/roadmap.md`](./docs/roadmap.md) for the full list of planned utilities
(region extraction, DeepZoom export, HTTP tile server, DICOM-WSI conversion, more
codecs, slide inventory/diff/verify, etc.) and architectural items still queued.

## Build prerequisites

cgo dependencies (macOS via Homebrew):

```sh
brew install jpeg-turbo openjpeg jpeg-xl libavif webp openjph
```

`pkg-config` resolves all of them at build time. Linux equivalents (Debian/Ubuntu):

```sh
apt install libturbojpeg0-dev libopenjp2-7-dev libjxl-dev libavif-dev libwebp-dev
# OpenJPH (HTJ2K) typically requires source build on Linux as of 2026-05.
```

Build a slim binary that skips selected codecs via build tags:

```sh
go build -tags 'noavif nowebp nohtj2k' ./cmd/wsi-tools   # only JPEG-XL + JPEG
go build -tags 'nojxl noavif nowebp nohtj2k' ./cmd/wsi-tools   # only JPEG (v0.1 surface)
```

## Install

```sh
go install github.com/cornish/wsi-tools/cmd/wsi-tools@latest
```

## Usage

### Downsample

```sh
# 40x SVS → 20x SVS (factor 2 default)
wsi-tools downsample -o slide-20x.svs slide-40x.svs

# 40x → 10x with --factor 4 at higher quality
wsi-tools downsample --factor 4 --quality 95 -o slide-10x.svs slide-40x.svs

# Or via target magnification
wsi-tools downsample --target-mag 10 -o slide-10x.svs slide-40x.svs
```

### Transcode

```sh
# SVS to JPEG-XL (output is a generic pyramidal TIFF with WSIImageType-tagged IFDs)
wsi-tools transcode --codec jpegxl -o slide-jxl.tiff slide.svs

# SVS re-encoded as JPEG at a different quality (still SVS-shaped)
wsi-tools transcode --codec jpeg --quality 75 -o slide-q75.svs slide.svs

# AVIF with a faster encoder preset
wsi-tools transcode --codec avif --codec-opt avif.speed=8 -o out.tiff in.svs

# Lossless WebP for archival
wsi-tools transcode --codec webp --codec-opt webp.lossless=true -o out.tiff in.svs

# HTJ2K
wsi-tools transcode --codec htj2k -o out.tiff in.svs
```

### Inspection

```sh
# Slide summary (analog of openslide-show-properties)
wsi-tools info slide.svs

# Same data as JSON for scripting
wsi-tools info --json slide.svs | jq .levels

# Format-aware per-IFD layout dump
wsi-tools dump-ifds slide.svs

# Save the slide's label as a standalone PNG
wsi-tools extract --kind label -o label.png slide.svs

# Content hash for cache identity / dedup (default: SHA-256 of file bytes)
wsi-tools hash slide.svs

# Pixel-stable hash (decodes L0 tiles → SHA-256 of RGB raster)
wsi-tools hash --mode pixel slide.svs
```

### Other

```sh
# Check installed codec libs
wsi-tools doctor

# Suppress progress bar (useful in CI / scripts)
wsi-tools --quiet downsample -o out.svs in.svs

# Per-level timing summaries on stderr
wsi-tools --verbose downsample -o out.svs in.svs

# Structured JSON logging (for log aggregators)
wsi-tools --log-format json downsample -o out.svs in.svs
```

### Example output

```
$ wsi-tools downsample -o CMU-1-Small-Region-10x.svs CMU-1-Small-Region.svs
encoding  100% 30/30 tiles  742 tiles/s  ETA 0s
wrote CMU-1-Small-Region-10x.svs (1.0 MB, 39ms)
```

## Memory

`downsample` (v0.1) still holds the full L0 raster in memory during pyramid build:

- A 20x slide L0: ~50K × 30K × 3 ≈ 4.5 GB
- A 40x slide L0: ~100K × 60K × 3 ≈ 18 GB

This fits on most workstations but is tight on laptops. `transcode` (v0.2+) streams
per-tile and has a constant-memory ceiling regardless of slide size. A streaming
retrofit for `downsample` is queued — see [`docs/roadmap.md`](./docs/roadmap.md).

## Testing

```sh
make test     # unit tests, race-detector
make vet
```

Integration tests run against real SVS fixtures, gated by `WSI_TOOLS_TESTDIR`:

```sh
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -v -timeout 60m
```

`sample_files/` in this repo is gitignored; soft-link to your fixture pool:

```sh
ln -s $HOME/GitHub/opentile-go/sample_files sample_files
```

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
