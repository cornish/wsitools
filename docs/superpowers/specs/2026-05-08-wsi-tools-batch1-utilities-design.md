# wsi-tools v0.4 — batch 1 inspection utilities

**Status:** Sealed for implementation
**Date:** 2026-05-08
**Predecessors:** [v0.2 transcode](2026-05-07-wsi-tools-v02-design.md), [v0.3 opentile-go v0.14](2026-05-08-wsi-tools-v03-opentile-v014-design.md)

## Goal

Ship four read-side CLI utilities — analogs of openslide-tools and tiffinfo —
that turn wsi-tools from a write-only toolset (downsample, transcode) into
something useful for inspecting and scripting against existing slides.

The four utilities:

- **`info`** — slide summary; analog of `openslide-show-properties`.
- **`dump-ifds`** — format-aware per-IFD layout dump (not a full tiffinfo
  replacement; tiffinfo continues to own raw-tag dumps for now).
- **`extract`** — save an associated image (label / macro / thumbnail /
  overview) as PNG or JPEG.
- **`hash`** — content hash; analog of `openslide-quickhash1`. File mode
  by default; pixel mode opt-in.

Plus one shared package and one new docs file:

- **`internal/cliout`** — text/JSON dual-rendering helpers used by all four
  utilities. Eliminates per-subcommand format-flag boilerplate.
- **`docs/roadmap.md`** — durable record of the full utilities roadmap
  (batch 1, planned batch 2, plus the larger items: dzsave, tile-server,
  DICOM-WSI conversion, tagset, inventory, verify, diff). Lives at the top
  of `docs/` so it's discoverable; not under `docs/superpowers/`.

## Non-goals

- **Full TIFF tag dump** (`tiffinfo` replacement) — `dump-ifds` is the
  format-aware classifier flavor only. A future `--raw` flag will expand
  it; reserved as roadmap work.
- **Region extraction** (`openslide-write-png` for arbitrary
  `--x --y --w --h --level` rectangles) — useful but more work; deferred
  to batch 2.
- **DeepZoom export, HTTP tile server, DICOM-WSI conversion, in-place tag
  editing, slide-pool inventory, verify, diff** — all tracked in
  `docs/roadmap.md`; out of scope for v0.4.0.

## Cross-cutting design

### CLI surface

Each utility is a cobra subcommand registered in its own file under
`cmd/wsi-tools/`. One file per command, matching the existing pattern
(`doctor.go`, `downsample.go`, `transcode.go`, `version.go`).

Every utility supports `--json` to emit machine-readable JSON instead of
human-readable text. `--json` is bound via `internal/cliout` so all four
share the flag definition + render path.

### `internal/cliout` package

Three responsibilities:

1. **JSON encoding.** A `JSON(w io.Writer, v any) error` helper that
   marshals with `json.MarshalIndent` (2-space indent, trailing newline)
   and writes to `w`.
2. **`--json` flag binding.** A helper that registers the `--json` bool
   flag on a `*cobra.Command` and returns a pointer the subcommand reads.
3. **Dual-rendering dispatch.** A `Render(jsonMode bool, w io.Writer,
   human func(io.Writer) error, machine any) error` helper that branches
   on the flag and runs the right path. Subcommands construct their
   typed result, then call `Render`.

Surface, in full:

```go
package cliout

func RegisterJSONFlag(cmd *cobra.Command) *bool

func Render(jsonMode bool, w io.Writer,
    human func(io.Writer) error, machine any) error

func JSON(w io.Writer, v any) error
```

That's it. No tables, no fancy template engine. Each subcommand writes
its own `human` closure — they're 10–30 lines of `fmt.Fprintf` each, and
inlining keeps the human format under each subcommand's local
control without forcing a generic "Table" abstraction prematurely.

### Error model

Same pattern as `transcode`:

- Source-format-unsupported (NDPI, OME-OneFrame, Leica SCN at v0.4) →
  clean message, exit 1.
- Source-compression-undecodable (pixel hash on a WebP/JXL/AVIF/HTJ2K
  source — rare in scanner outputs) → clean message, exit 2.
- I/O errors propagate with `%w` wrapping; cobra prints them.
- No panics on bad input.

### Testing

- **Unit tests** per package (`internal/cliout` — JSON shape, render
  dispatch). Hash file/pixel modes get unit tests against synthetic
  fixed-bytes input.
- **Integration tests** under `tests/integration/` (`-tags integration`)
  per utility, gated by `WSI_TOOLS_TESTDIR`. Each runs the built binary
  against a known fixture and asserts on stdout shape (substring
  matches for human output, JSON-decode + field assertions for
  `--json` output).
- **Golden file** style is tempting for `info` / `dump-ifds` text
  output, but acquired-date drift and platform-dependent file sizes
  make exact-string golden too brittle. Prefer substring assertions on
  stable fields (`"Format:    svs"`, level count, presence of expected
  associated images).

## Per-utility design

### `wsi-tools info <file>`

Slide summary, openslide-show-properties analog.

#### Human output

```
File:    slide.svs (1.2 GB)
Format:  svs
Make:    Aperio
Model:   ScanScope ...
Software: Aperio ScanScope ScanScope - 102.0.7.5
DateTime: 2026-04-15 13:14:15
MPP:     0.2520
Magnification: 40x

Levels:
  L0  46720 × 32914   tile 240×240   jpeg
  L1  11680 ×  8228   tile 240×240   jpeg
  L2   2920 ×  2057   tile 240×240   jpeg

Associated images:
  thumbnail  574 × 768    jpeg
  label      387 × 463    lzw
  macro     1280 × 431    jpeg
```

Optional fields (Make, Model, Software, DateTime, MPP, Magnification,
Associated images) are omitted entirely if empty/zero — no `(unknown)`
filler.

#### JSON output (`--json`)

```json
{
  "path": "slide.svs",
  "size_bytes": 1289401234,
  "format": "svs",
  "metadata": {
    "make": "Aperio",
    "model": "ScanScope ...",
    "software": "Aperio ScanScope ScanScope - 102.0.7.5",
    "datetime": "2026-04-15T13:14:15Z",
    "mpp": 0.2520,
    "magnification": 40
  },
  "levels": [
    {"index": 0, "width": 46720, "height": 32914,
     "tile_width": 240, "tile_height": 240, "compression": "jpeg"},
    {"index": 1, "width": 11680, "height": 8228,
     "tile_width": 240, "tile_height": 240, "compression": "jpeg"},
    {"index": 2, "width": 2920, "height": 2057,
     "tile_width": 240, "tile_height": 240, "compression": "jpeg"}
  ],
  "associated_images": [
    {"kind": "thumbnail", "width": 574, "height": 768, "compression": "jpeg"},
    {"kind": "label",     "width": 387, "height": 463, "compression": "lzw"},
    {"kind": "macro",     "width": 1280, "height": 431, "compression": "jpeg"}
  ]
}
```

Empty-string and zero metadata fields are emitted as empty/zero, NOT
omitted (JSON consumers prefer fixed schema).

#### Implementation

Reads `internal/source.Source`. No new package code beyond the
subcommand file itself. ~150 lines.

### `wsi-tools dump-ifds <file>`

Format-aware per-IFD layout dump.

#### Default human output (option α — sealed)

```
IFD 0  pyramid L0  46720 × 32914   tile 240×240  jpeg     SubfileType=0
IFD 1  thumbnail     574 × 768                   jpeg     SubfileType=1
IFD 2  pyramid L1  11680 × 8228    tile 240×240  jpeg     SubfileType=0
IFD 3  pyramid L2   2920 × 2057    tile 240×240  jpeg     SubfileType=0
IFD 4  label         387 × 463                   lzw      SubfileType=1
IFD 5  macro        1280 × 431                   jpeg     SubfileType=9
```

Followed by an optional WSI-tags section if any of our private tags
(65080–65084) are present:

```
WSI tags (private 65080–65084):
  WSIImageType    = pyramid    (on IFD 0, 2, 3)
  WSILevelIndex   = 0/1/2      (on IFD 0, 2, 3)
  WSILevelCount   = 3          (on IFD 0)
  WSISourceFormat = svs        (on IFD 0)
  WSIToolsVersion = 0.3.1      (on IFD 0)
```

#### `--raw` flag (deferred — NOT in v0.4.0)

The `--raw` flag is reserved for a future expansion that does a full
tiffinfo-style tag dump per IFD. Documented in `docs/roadmap.md` as
"dump-ifds raw mode." Not exposed in v0.4.0; calling `dump-ifds --raw`
returns "unknown flag." Once batch 2 (or a v0.4.x patch) ships it, the
flag becomes available.

This explicit non-implementation in v0.4.0 keeps the help text honest
and avoids the half-implemented-flag anti-pattern.

#### IFD-order vs classification-order

Important detail: the output uses **IFD order** (file order), NOT
classification order. This is what makes the tool useful — it shows
where each IFD lives in the file, so users can correlate with `tiffinfo`
output, observe SubfileType / wsi-tools tag annotations, and understand
the actual layout.

Implementation note: opentile-go's `Levels()` returns levels in
classification order (L0 first); `Associated()` returns associated
images. To get IFD order, we need a small TIFF IFD walker. The
`internal/source/imagedesc.go::ReadSourceImageDescription` helper
already does the start of this work (header parse + IFD chain walk for
tag 270). Extend it (or factor a new `internal/source/ifdwalk.go`) to
return per-IFD records: index, width/height, tile size (if tiled),
compression tag, NewSubfileType, ImageDescription excerpt, and any
private wsi-tools tags found.

Then cross-reference each IFD against opentile-go's classification
output to label it (pyramid L0/L1/.../label/macro/thumbnail/overview/
probability/map). When opentile-go doesn't classify an IFD (e.g.,
JPEGTables-only IFDs or padding), the dump labels it `(unclassified)`.

#### JSON output (`--json`)

```json
{
  "path": "slide.svs",
  "format": "svs",
  "ifds": [
    {"index": 0, "kind": "pyramid", "level_index": 0,
     "width": 46720, "height": 32914,
     "tile_width": 240, "tile_height": 240,
     "compression": "jpeg", "compression_tag": 7,
     "subfile_type": 0,
     "wsi_tags": {"WSIImageType": "pyramid", "WSILevelIndex": 0,
                  "WSILevelCount": 3, "WSISourceFormat": "svs",
                  "WSIToolsVersion": "0.3.1"}},
    {"index": 1, "kind": "thumbnail", "width": 574, "height": 768,
     "tile_width": 0, "tile_height": 0, "compression": "jpeg",
     "compression_tag": 7, "subfile_type": 1, "wsi_tags": null}
  ]
}
```

`tile_width`/`tile_height` = 0 indicates the IFD is stripped (not tiled,
typical for associated images). `wsi_tags` = null when no private tags
present.

#### Implementation

New file `internal/source/ifdwalk.go` (~150 lines) extending the
imagedesc helper to a full IFD walker that returns per-IFD records
including the wsi-tools private tags. Subcommand file is the dispatcher
+ classifier crossref + render. ~200 lines for the subcommand.

### `wsi-tools extract <file>`

Save an associated image as PNG or JPEG.

```
$ wsi-tools extract --kind label slide.svs -o label.png
wrote label.png (15 KB)

$ wsi-tools extract --kind macro slide.svs -o macro.jpg --format jpeg
wrote macro.jpg (87 KB)
```

#### Flags

- `--kind {label|macro|thumbnail|overview}` (required) — must match a
  Kind in `Source.Associated()`.
- `-o PATH` (required) — output file path.
- `--format {png|jpeg}` (default `png`).

#### Behaviour

For `--format png`:
- Decode the associated image's bytes via `internal/decoder` (jpeg or
  jpeg2000). LZW (sometimes used for labels) decodes via Go's
  `golang.org/x/image/tiff` package, already a transitive dep through
  `golang.org/x/image v0.39.0`. If a label uses an unrecognised
  compression, error cleanly with the codec's name.
- Re-encode to PNG via Go stdlib `image/png`.

For `--format jpeg`:
- If the source associated image is already JPEG, **byte-pass-through**:
  copy the original bytes verbatim. No re-encode loss.
- Otherwise (LZW label, etc.), decode + re-encode to JPEG via Go stdlib
  `image/jpeg` at quality 90.

If the requested `--kind` isn't present in the source, error with the
list of available kinds.

#### Implementation

Pure orchestration on top of existing `Source.Associated()` +
`internal/decoder` + `image/png` + `image/jpeg`. ~150 lines.

### `wsi-tools hash <file>`

Content hash, openslide-quickhash1 analog.

```
$ wsi-tools hash slide.svs
sha256:1a2b3c... slide.svs

$ wsi-tools hash --mode pixel slide.svs
sha256-pixel:9f8e7d... slide.svs
```

#### Flags

- `--mode {file|pixel}` (default `file`).

#### Behaviour

**`--mode file`** (default): `io.Copy(hasher, f)` over the input file →
SHA-256 hex. Works for every format. Cheap.

**`--mode pixel`**: walk L0 tiles in raster order, decode each to RGB
via `internal/decoder`, feed RGB bytes into a single SHA-256 hasher.
Stable across re-encode at the same nominal quality. Errors cleanly if
the L0 compression isn't decodable (WebP / JXL / AVIF / HTJ2K — rare in
scanner outputs but possible in transcoded inputs).

The L0-not-smallest choice deliberately sidesteps the SZI / DZI 1×1
smallest-level degenerate case.

The pixel-hash byte order: for each tile in raster (row-major, left to
right, top to bottom) order, append its decoded RGB bytes to the
hasher. Tile bytes are written as `RGBRGBRGB...` row by row within the
tile. No edge-tile padding adjustment — the source tile's full extent
is hashed, including any padding the source format includes.

This isn't byte-for-byte compatible with openslide's own quickhash1
algorithm (which is per-format and includes additional canonicalization
steps). Documented as "wsi-tools pixel hash, not openslide-compatible."

#### Output

Human (no `--json`):
```
sha256:<hex> <path>          # file mode
sha256-pixel:<hex> <path>    # pixel mode
```

JSON (`--json`):
```json
{
  "algorithm": "sha256",
  "mode": "file",   // or "pixel"
  "hex": "1a2b...",
  "path": "slide.svs"
}
```

#### Implementation

Minimal. Pure orchestration. ~120 lines.

## Documentation deliverable: `docs/roadmap.md`

Single new top-level docs file. Sections:

```
# wsi-tools utilities roadmap

## Shipped

### v0.1
- downsample, doctor, version

### v0.2
- transcode

### v0.3
- (no new utilities — opentile-go migration milestone)

### v0.4 (this milestone)
- info, dump-ifds, extract, hash

## Planned

### Batch 2 — extends batch 1
- region (openslide-write-png analog: --x --y --w --h --level → PNG)
- dump-tile (single tile compressed bytes to file/stdout, debug aid)
- dump-ifds --raw (full tiffinfo-style tag dump per IFD)

### Batch 3 — operations
- tagset (in-place TIFF tag edit, e.g. ImageDescription, Software)
- inventory (walk a directory, dump CSV/JSON of slide metadata for pool
  management UIs)
- verify (open every IFD, decode every tile, report errors — "fsck for
  WSI")
- diff (compare two slides — pixel diff, metadata diff, IFD ordering)

### Larger items
- dzsave (DeepZoom pyramid generator — `dzsave` / `deepzoom_tile.py`
  analog; outputs OpenSeadragon-compatible tile tree)
- tile-server (HTTP DZI/IIIF tile server — `deepzoom_server.py` analog;
  activates opentile-go v0.13 splice-prefix optimization)
- dicom-wsi (convert WSI to DICOM-WSI format — `wsi2dcm` /
  `wsidicomizer` analog)

## Codecs

(These are write-side, separate from read-side utilities.)

### Deferred from v0.2
- jpegli, HEIF, JPEG-LS, JPEG-XR, Basis Universal
- jpeg2000 as transcode-encoder target

## Source format support

### Deferred from v0.2
- Leica SCN (multi-image / multi-channel)

## Architectural

### Deferred from v0.2
- Streaming retrofit for downsample (currently materializes full L0
  raster)

### Deferred from v0.3
- TilePrefix / TileBodyInto / SpliceJPEGTile adoption (only valuable
  if tile-server is built)

## Quality gates

### Deferred from v0.2
- Visual-fidelity tests via mini decoders (decode our codec outputs
  through matching codec library; pixel-compare against source)
```

## Versioning

Target **v0.4.0**. Pure-feature minor release: four new subcommands,
one new internal package, one new docs file. No breaking changes.

## Risks

- **`dump-ifds` IFD walker correctness.** Walking IFDs across the
  full WSI fixture pool (SVS, Philips-TIFF, OME-TIFF, BIF, IFE,
  generic-TIFF, BigTIFF) is meaningful work. The ifdwalk helper needs
  to handle ClassicTIFF + BigTIFF headers, regular IFDs + SubIFDs (used
  by OME-TIFF), and at least the tags we care about (256/257 dimensions,
  259 compression, 254 NewSubfileType, 322/323 tile size, 270
  ImageDescription, 65080–65084 wsi-tools privates). Mitigation:
  integration test exercises every fixture; classifier cross-reference
  catches drift between our walk and opentile-go's reading.
- **`extract --format jpeg` byte-pass-through.** Source JPEG bytes
  (raw associated image) may include APP markers or other quirks that
  Go stdlib's `image/jpeg` decode chain rejects. Mitigation: pass-through
  is a copy, not a decode/re-encode — so as long as the source bytes
  are something a viewer accepts, our output is the same bytes. We
  don't need to validate.
- **`hash --mode pixel` byte order stability.** Future changes to tile
  iteration or padding handling would silently invalidate previously
  computed hashes. Mitigation: document the algorithm precisely (raster
  order, row-major tile bytes, no padding adjustment) and include the
  algorithm name in the output (`sha256-pixel:`) so any future
  algorithm change can use a different prefix.
- **`internal/cliout` over-engineering.** This package is small on
  purpose. Keep it that way — table renderers, alignment helpers, etc.
  are YAGNI for v0.4. Prose-format-each-subcommand-itself is fine.
