# wsi-tools

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

A Swiss-army knife of utilities for whole-slide imaging (WSI) files used in digital pathology.

## v0.1 — what's here

- `wsi-tools downsample` — downsample a WSI by a power-of-2 factor (e.g. 40x → 20x). Regenerates the full pyramid from the new base; passes through associated images verbatim. SVS-only at v0.1.

## Future

- `wsi-tools transcode` — re-encode a WSI in alternative codecs (JPEG-XL, AVIF, WebP, jpegli, HEIF, JPEG-LS, JPEG-XR, HTJ2K, Basis Universal). v0.2.
- NDPI / Philips / OME-TIFF / BIF / IFE source support. v0.2+.
- GUI front-end. v0.4+.
- Absorption of `cornish/wsi-label-tools` (label remove / replace).

## Install

```sh
go install github.com/cornish/wsi-tools/cmd/wsi-tools@latest
```

## Build prerequisites

cgo dependencies (macOS via Homebrew):

```sh
brew install jpeg-turbo openjpeg
```

`pkg-config` resolves both at build time.

## Usage

```sh
# 40x SVS → 20x SVS
wsi-tools downsample -o slide-20x.svs slide-40x.svs

# Check installed codec libs
wsi-tools doctor
```

## License

Apache 2.0. See `LICENSE`.
