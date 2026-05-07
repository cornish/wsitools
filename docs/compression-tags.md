# wsi-tools — TIFF Compression tag values

TIFF tag 259 (`Compression`) values used by wsi-tools when writing tiled pyramidal
TIFF / SVS output. Standard values come from ISO, Adobe, or community allocation;
private values (≥ 32768) are wsi-tools-assigned for codecs that lack a recognized
tag value.

## Standard / community-allocated

| Codec | Tag | Source | Notes |
|---|---|---|---|
| None / uncompressed | 1 | TIFF 6.0 | |
| LZW | 5 | TIFF 6.0 | Used by Aperio for label associated images. |
| JPEG | 7 | TIFF 6.0 (Tech Note 2) | "New-style" JPEG-in-TIFF; not "OJPEG" (6). |
| Deflate | 8 | TIFF 6.0 + community | Adobe-allocated. |
| JPEG 2000 (Aperio) | 33003 / 33005 | Aperio | 33003 = YCbCr; 33005 = RGB. Aperio-private. |
| JPEG-LS | 34712 | ISO/IEC 14495 | Standard. |
| WebP | 50001 | Adobe | Adobe-allocated; libtiff supports. |
| JPEG-XL | 50002 | Adobe (draft) | Allocated; spec finalising. |

## wsi-tools private (≥ 32768)

| Codec | Tag | Notes |
|---|---|---|
| AVIF | 60001 | No standard TIFF tag. |
| HEIF | 60002 | No standard TIFF tag. |
| HTJ2K | 60003 | Could overlap JP2K (33003/33005); private to disambiguate. |
| JPEG-XR | 60004 | Microsoft has historically used 22610 for HD Photo; we're not bound to that. |
| Basis Universal | 60005 | Wrapped in KTX2 inside the tile bytes. |

These private values are only readable by wsi-tools-aware viewers and decoders.
This is by design — the `transcode` tool's purpose is to feed test fixtures into
viewers that understand these codecs natively.

## Verification of standard values

Pinned against:
- `libtiff` `libtiff/tif_dir.h` (COMPRESSION_* constants).
- AwareSystems TIFF tag reference (https://www.awaresystems.be/imaging/tiff/tifftags/compression.html).

When opening any newly-written TIFF in `tiffinfo`, the Compression line should
match these values exactly.
