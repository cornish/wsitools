# wsi-tools — viewer compatibility checklist

Manual checklist of (output codec, viewer) pairs that have been verified to load.
Not in CI; run by hand and update this file when you confirm a pair works.

## v0.1 — downsample tool

| Codec | Viewer | Verified? | Notes |
|---|---|---|---|
| JPEG (Aperio SVS) | QuPath | — | |
| JPEG (Aperio SVS) | openslide-bin | — | |
| JPEG (Aperio SVS) | OpenSeadragon (via DZI) | — | |

## v0.2 — transcode tool

Manual checklist of (output codec, viewer) pairs that have been verified to load.
Mark entries `✓` when confirmed by eye, `✗` if loading fails (with a note), `—`
if not yet tested.

The four v0.2 codecs (jpegxl, avif, webp, htj2k) write TIFF compression values
that opentile-go v0.12 doesn't yet decode (50001/50002/60001/60003). They will
need either an opentile-go release that adds these compression IDs OR a
viewer that owns its own libjxl / libavif / libwebp / OpenJPH decode path.

| Codec   | QuPath | openslide | Custom Viewer | OpenSeadragon |
|---------|--------|-----------|---------------|---------------|
| jpeg    | —      | —         | —             | —             |
| jpegxl  | —      | —         | —             | —             |
| avif    | —      | —         | —             | —             |
| webp    | —      | —         | —             | —             |
| htj2k   | —      | —         | —             | —             |

## v0.2.x deferred

- jpegli (Homebrew jpeg-xl bottle ships libjxl without libjpegli; defer until
  upstream re-enables or we stand up a build-from-source path).
- HEIF, JPEG-LS, JPEG-XR, Basis Universal: queued for follow-on releases.
- Visual-fidelity round-trip tests via mini decoders (read raw tile bytes
  from opentile-go, decode via the matching codec library).
