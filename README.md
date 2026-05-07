# wsi-tools

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

A Swiss-army knife of utilities for whole-slide imaging (WSI) files used in digital pathology.

## v0.1 — what's here

- `wsi-tools downsample` — downsample a WSI by a power-of-2 factor (e.g. 40x → 20x).
  Regenerates the full pyramid from the new base. Passes through associated images
  (label, macro, thumbnail, overview) verbatim. SVS-only at v0.1.
- `wsi-tools doctor` — report installed codec libraries.
- `wsi-tools version` — print version + Go runtime info.

## Future

- `wsi-tools transcode` — re-encode a WSI in alternative codecs (JPEG-XL, AVIF, WebP, jpegli, HEIF, JPEG-LS, JPEG-XR, HTJ2K, Basis Universal). v0.2.
- NDPI / Philips / OME-TIFF / BIF / IFE source support. v0.2+.
- Streaming pyramid build (current v0.1 holds full L0 raster in memory). v0.2.
- Lanczos resampler for non-power-of-2 factors. v0.2.
- GUI front-end. v0.4+.
- Absorption of `cornish/wsi-label-tools` (label remove / replace).

## Build prerequisites

cgo dependencies (macOS via Homebrew):

```sh
brew install jpeg-turbo openjpeg
```

`pkg-config` resolves both at build time. Linux equivalents: `apt install libturbojpeg0-dev libopenjp2-7-dev`.

## Install

```sh
go install github.com/cornish/wsi-tools/cmd/wsi-tools@latest
```

## Usage

```sh
# 40x SVS → 20x SVS (factor 2 default)
wsi-tools downsample -o slide-20x.svs slide-40x.svs

# 40x → 10x with --factor 4 at higher quality
wsi-tools downsample --factor 4 --quality 95 -o slide-10x.svs slide-40x.svs

# Or via target magnification
wsi-tools downsample --target-mag 10 -o slide-10x.svs slide-40x.svs

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

v0.1 holds the full L0 raster in memory during pyramid build:

- A 20x slide L0: ~50K × 30K × 3 ≈ 4.5 GB
- A 40x slide L0: ~100K × 60K × 3 ≈ 18 GB

This fits on most workstations but is tight on laptops. The `svs_40x_bigtiff.svs`
fixture (~4.8 GB on disk, ~18 GB decoded) is excluded from the integration sweep
for this reason. Streaming pyramid build is planned for v0.2.

## Testing

```sh
make test     # unit tests, race-detector
make vet
```

Integration tests run against real SVS fixtures, gated by `WSI_TOOLS_TESTDIR`:

```sh
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -v -timeout 30m
```

`sample_files/` in this repo is gitignored; soft-link to your fixture pool:

```sh
ln -s $HOME/GitHub/opentile-go/sample_files sample_files
```

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
