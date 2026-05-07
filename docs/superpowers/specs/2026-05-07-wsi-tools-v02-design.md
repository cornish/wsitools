# wsi-tools v0.2 — design

**Status:** draft, awaiting user review
**Date:** 2026-05-07
**Author:** brainstormed with Claude Opus 4.7
**Module:** `github.com/cornish/wsi-tools`
**Predecessor spec:** `docs/superpowers/specs/2026-05-06-wsi-tools-v01-design.md`

## Goal

Ship `wsi-tools transcode` end-to-end on a useful subset of WSI source formats and target codecs, with a streaming pyramid build that lifts the v0.1 memory ceiling. v0.2 is split into a v0.2.0 baseline release plus follow-on v0.2.1+ codec additions.

**v0.2.0 baseline (this spec):**

- `wsi-tools transcode` subcommand (cobra).
- **5 new codec wrappers**: **JPEG-XL** (libjxl), **jpegli** (libjxl), **AVIF** (libavif), **WebP** (libwebp), **HTJ2K** (OpenJPH). The `jpeg` codec from v0.1 also becomes available as a transcode target for free (re-encode at a different `--quality`), bringing the transcode CLI's `--codec` flag to 6 accepted values total.
- 6 sane source formats: **SVS, Philips-TIFF, OME-TIFF (tiled SubIFD path), BIF, IFE, generic-TIFF**. NDPI, OME-OneFrame, and Leica SCN reject cleanly with `ErrUnsupportedFormat` (Leica SCN's multi-image + multi-channel structure is out of scope for v0.2.0's single-pyramid streaming model).
- **Streaming pyramid build** — per-tile decode → encode → write, no L0 raster materialisation. Memory ceiling drops three orders of magnitude vs. v0.1 downsample.
- New private TIFF tags `WSIImageType` (65080) + `WSILevelIndex` (65081) + `WSILevelCount` (65082) + `WSISourceFormat` (65083) + `WSIToolsVersion` (65084) for self-describing output.
- Standard TIFF metadata tags (Make / Model / Software / DateTime) populated from opentile-go's cross-format metadata.
- v0.1 downsample IFD-ordering bug fix bundled in (associated images currently land in the wrong order for opentile-go's SVS classifier; corrected as part of the v0.2 writer changes).

**Deferred to v0.2.1+:**

- Codecs beyond the v0.2.0 five: HEIF, JPEG-LS, JPEG-XR, Basis Universal, plus jpeg / jpeg2000 as transcode targets (their decoders are already shipped at v0.1).
- Streaming retrofit for `downsample` (fast-follow once the streaming pattern is proven on transcode).
- `--re-encode-associated` flag (v0.2 default is verbatim passthrough).
- Format-native metadata blob preservation (Philips XML, OME-XML, vendor tags).

**Still v0.3+:**

- libjpeg `longjmp` error recovery.
- Linux CI lane.
- NDPI + OME-OneFrame source support (would need a virtual-tile materialisation path; out of scope for v0.2's "preserve source tile geometry" model).
- GUI front-end.
- `cornish/wsi-label-tools` absorption.

## Non-goals (v0.2.0)

- Cross-codec round-trip (encode in jpegxl → decode → re-encode in avif). Multi-generation loss is real but not v0.2's problem.
- Per-codec parity oracles against reference encoders' byte sequences. Too brittle; PSNR > threshold is sufficient.
- Linux / Windows test runs (Windows is build-only).
- Re-tiling: output tile dimensions = source tile dimensions, level-by-level, verbatim.

## Constraints / decisions made during brainstorming

- **Reader = `github.com/cornish/opentile-go` v0.12.x.** v0.12 renames the `philips` and `ome` format strings to disambiguate variants (now `philips-tiff` and `ome-tiff`). v0.11 also added `FormatLeicaSCN`. wsi-tools references opentile-go's exported `Format*` constants (`opentile.FormatSVS`, `opentile.FormatPhilipsTIFF`, `opentile.FormatOMETIFF`, `opentile.FormatBIF`, `opentile.FormatIFE`, `opentile.FormatGenericTIFF`, `opentile.FormatNDPI`, `opentile.FormatLeicaSCN`) rather than literal strings, so future renames stay mechanical.
- **Source-format sanity gate** lives in a new `internal/source` package. Reject NDPI outright (no source tile geometry — striped MCU streams). Reject OME-TIFF when the OneFrame path is detected (same shape as NDPI's striped layout — synthesised tiles, not stored ones). Reject Leica SCN at v0.2.0 (multi-image + multi-channel structure requires per-Image / per-channel pipeline plumbing we don't ship until v0.2.x).
- **BIF is in.** Serpentine remap + L0 horizontal overlap are real but opentile-go normalises the read interface; we just preserve the source's tile geometry verbatim.
- **Streaming end-to-end.** Transcode has no inter-level dependency, so each level is a streamed tile pass: decode → encode → write. No raster intermediate. Memory ceiling: `~workers × tile_bytes × 2` (decoded + encoded buffers per worker), independent of slide size.
- **Output container per source/codec:**
  - **SVS source + jpeg or jpegli codec** → SVS-shaped output (Aperio convention; jpegli output is standard JPEG bitstream so opentile-go's SVS reader picks it up unchanged).
  - **Everything else** → generic pyramidal TIFF output (Compression tag varies per codec; opentile-go's generic-TIFF reader handles re-open).
- **IFD ordering:**
  - **SVS-shaped output:** match Aperio's exact convention — `L0, thumbnail, L1, L2, …, LN, label, macro`. opentile-go's `formats/svs/series.classifyPages` requires this layout to classify trailing pages correctly. Bundle the v0.1 downsample fix here (downsample currently writes `L0..LN, thumbnail, label, macro` which round-trips ambiguously).
  - **Generic-TIFF output:** clean ordering — `L0, L1, …, LN, all associated images at end` in whatever order opentile-go's `Source.Associated()` returned them. `WSIImageType` (tag 65080) is the authoritative classifier; opentile-go's generic-TIFF heuristic remains the fallback.
- **Tile geometry preservation.** Output tile dimensions = source tile dimensions, level-by-level. No re-tiling. This is what makes streaming work.
- **Per-codec build tags** (`-tags nojxl noavif nowebp nohtj2k nojpegli`). Default = all codecs; slim builds skip the corresponding `init()` registration. `wsi-tools doctor` reports honestly which codecs are linked.
- **Quality knobs:** single `--quality 1..100` mapped per-codec inside each wrapper; `--codec-opt key=value` (repeatable) for codec-specific tuning. Codec-opt keys are namespaced per codec (`jxl.distance`, `avif.speed`, `webp.lossless`, etc.).
- **Atomic output.** `<output>.tmp` + `rename(2)` on success; tmp removed on error or SIGINT. Same as v0.1.

## Repo layout (delta from v0.1)

```
wsi-tools/
├── cmd/wsi-tools/
│   ├── transcode.go            # NEW: transcode subcommand
│   ├── downsample.go           # extended: pin opentile-go Format* constants;
│   │                           # fix associated-image IFD ordering
│   └── (doctor.go, version.go, main.go untouched)
├── internal/
│   ├── source/                 # NEW: source-format adapter
│   │   ├── source.go           # Source / Level / AssociatedImage interfaces; Open()
│   │   ├── opentile.go         # opentile-go Tiler wrapper + sanity gate
│   │   └── imagedesc.go        # tag-270 reader (promoted from cmd/wsi-tools/downsample.go)
│   ├── wsiwriter/
│   │   ├── tiff.go             # extended: WithMake/Model/Software/DateTime/Source/Version options
│   │   ├── wsitags.go          # NEW: WSIImageType/WSILevelIndex/etc. tag constants + emission helpers
│   │   └── (svs.go, jpegtables.go, compression.go untouched)
│   ├── codec/
│   │   ├── jpegli/             # NEW: libjxl/jpegli encoder (libjpeg-API-compatible)
│   │   ├── jpegxl/             # NEW: libjxl encoder
│   │   ├── avif/               # NEW: libavif encoder
│   │   ├── webp/               # NEW: libwebp encoder
│   │   ├── htj2k/              # NEW: OpenJPH encoder
│   │   ├── all/all.go          # extended: blank-import the 5 new codecs
│   │   └── (codec.go, jpeg/ untouched)
│   └── (decoder, pipeline, resample untouched)
├── docs/
│   ├── tiff-tags.md            # RENAMED from compression-tags.md; covers WSI tag block
│   ├── viewer-compat.md        # extended with codec/viewer matrix
│   └── superpowers/specs/      # this v0.2 spec lands here
├── tests/
│   ├── integration/
│   │   ├── downsample_test.go  # extended: assert associated-image Kind() round-trip
│   │   └── transcode_test.go   # NEW: per-codec + per-source-format sweeps
│   └── bench/
│       └── transcode_bench_test.go  # NEW (optional)
└── (Makefile, README.md, CLAUDE.md, LICENSE, go.mod untouched apart from opentile-go version bump)
```

## Architecture

### Data flow

```
source.Tile bytes ──► decode ──► (no-op pixels) ──► encode ──► wsiwriter.WriteTile
   (per source tile)   (libjpeg-turbo or          (target codec        (one-to-one
                        OpenJPEG, both already    wrapper, new at      tile geometry)
                        shipped at v0.1)          v0.2)
```

#### Per-tile lifecycle

1. **Source read** — `source.Tile(level, x, y)` returns the raw compressed bytes opentile-go gave us. For TIFF-dialect sources, opentile-go's `Tile()` returns decoder-ready bytes (JPEGTables already spliced for SVS JPEG sources, per the v0.1 lesson). For IFE, the bytes are JPEG, AVIF, or Iris-proprietary — JPEG/AVIF supported via existing decoders; Iris-proprietary errors out.
2. **Decode** — dispatch on source `Compression()`:
   - JPEG → `internal/decoder.JPEG` (libjpeg-turbo). Full-resolution decode (no fast-scale needed for transcode).
   - JPEG 2000 → `internal/decoder.JPEG2000` (OpenJPEG).
   - **AVIF (IFE only)** → deferred to v0.2.1; v0.2.0 IFE source path supports JPEG only. AVIF-encoded IFE sources return `ErrUnsupportedSourceCompression` cleanly. Avoids pulling libavif into the decoder package's link surface (libavif lives only in `internal/codec/avif/` for encoding).
   - **Iris-proprietary** → `ErrUnsupportedSourceCompression`.
3. **Process** — pixels untouched. Identity step.
4. **Encode** — codec wrapper produces target-codec bytes.
5. **Sink** — `wsiwriter.LevelHandle.WriteTile(x, y, encoded)`, single writer goroutine owns the output file.

#### Per-pyramid-level lifecycle

Each source level transcodes one-to-one to the corresponding output level. **No inter-level dependency** (transcode is identity in the pixel domain), so:

- v0.2.0 serializes levels (one `pipeline.Run` per level). Simpler bookkeeping; level 0 dominates total work; per-tile worker-pool parallelism within a level is sufficient.
- All-levels-at-once parallelism is a v0.3 optimisation if profiling justifies it.

#### Concurrency

Worker pool of N decode→encode goroutines (default `runtime.NumCPU()`) feeding a single writer goroutine per output. Identical to v0.1's pipeline shape; `internal/pipeline` needs no changes.

#### Memory

`~workers × tile_bytes × 2` (decoded + encoded buffers per worker) plus opentile-go's mmap region (OS pages on demand → ~zero RSS impact). For 256×256 RGB tiles at NumCPU=10 workers: ~4 MB hot footprint. Slide size irrelevant — the 4.8 GB BigTIFF fixture (`svs_40x_bigtiff.svs`) re-enters the integration sweep at v0.2.0.

#### Associated images

Processed serially after the pyramid pass. Each associated image:

- **Kind passthrough**: take whatever opentile-go's `Source.Associated()` enumerated (label, macro, thumbnail, overview, probability, map, associated).
- **Bytes passthrough**: `a.Bytes()` returns a self-contained encoded blob; copy verbatim into the output IFD.
- **No re-encoding** at v0.2.0. `--re-encode-associated` flag deferred to v0.2.1+ if a real use case surfaces.

For SVS-shaped output, slot the associated images into Aperio's exact ordering: thumbnail right after L0 (interleaved into the pyramid IFD chain), label/macro at the end. For generic-TIFF output, all associated images go at the end of the IFD chain in opentile-go's enumeration order.

### Component: `internal/source`

A thin adapter between the CLI and opentile-go. Encapsulates the sanity gate, normalises the streaming interface, and is the single place where "what counts as a sane source" lives.

```go
package source

type Source interface {
    Format() string                 // matches opentile.Format* constants
    Levels() []Level                // pyramid levels in order, L0 first
    Associated() []AssociatedImage
    Metadata() Metadata
    SourceImageDescription() string // L0 raw ImageDescription if available; "" for IFE / IFE-like
    Close() error
}

type Level interface {
    Index() int
    Size() image.Point
    TileSize() image.Point
    Grid() image.Point
    Compression() Compression
    Tile(x, y int) ([]byte, error)
}

type AssociatedImage interface {
    Kind() string
    Size() image.Point
    Compression() Compression
    Bytes() ([]byte, error)
}

type Metadata struct {
    Make, Model, Software, SerialNumber string
    Magnification                       float64
    MPP                                 float64       // 0 if unknown
    AcquisitionDateTime                 time.Time
    Raw                                 map[string]string
}

func Open(path string) (Source, error)

var (
    ErrUnsupportedFormat = errors.New("source: format unsupported at v0.2 (NDPI and OME-OneFrame are skipped)")
    ErrUnsupportedSourceCompression = errors.New("source: source compression not decodable at v0.2")
)
```

**Sanity gate** in `internal/source/opentile.go`:

1. `t, _ := opentile.OpenFile(path)`.
2. Reject with `ErrUnsupportedFormat` when:
   - `t.Format() == opentile.FormatNDPI`, OR
   - `t.Format() == opentile.FormatOMETIFF` AND any pyramid level returns `Level.TileSize() == image.Point{}` (the OneFrame signal — striped IFDs report zero tile size).
3. Otherwise wraps each `opentile.Level` and `opentile.AssociatedImage` and returns a `Source`.

**Source ImageDescription extraction** lives in `internal/source/imagedesc.go`. Promoted from `cmd/wsi-tools/downsample.go`'s `readSourceImageDescription` helper (Task 22 of v0.1). Hand-rolled TIFF tag-270 reader covering both classic-TIFF and BigTIFF; returns `""` without error for non-TIFF sources (IFE).

### Component: `internal/wsiwriter` (extensions)

#### `wsitags.go` — WSI metadata tag block

```go
const (
    TagWSIImageType     uint16 = 65080  // ASCII; emitted on every IFD
    TagWSILevelIndex    uint16 = 65081  // LONG;  pyramid IFDs only
    TagWSILevelCount    uint16 = 65082  // LONG;  pyramid IFDs only
    TagWSISourceFormat  uint16 = 65083  // ASCII; L0 only
    TagWSIToolsVersion  uint16 = 65084  // ASCII; L0 only
)

// WSIImageType canonical values:
//   pyramid, label, macro, overview, thumbnail, probability, map, associated
const (
    WSIImageTypePyramid     = "pyramid"
    WSIImageTypeLabel       = "label"
    WSIImageTypeMacro       = "macro"
    WSIImageTypeOverview    = "overview"
    WSIImageTypeThumbnail   = "thumbnail"
    WSIImageTypeProbability = "probability"
    WSIImageTypeMap         = "map"
    WSIImageTypeAssociated  = "associated"
)
```

`LevelSpec` and `AssociatedSpec` gain an optional `WSI` field of a small embedded type that triggers tag emission. The writer's `Close()` walks each `imageEntry`, applies the WSI tag block where present, and emits in tag-sorted order alongside everything else.

#### `tiff.go` extensions

New options, additive (no breaking change to v0.1's surface):

```go
func WithMake(s string) Option
func WithModel(s string) Option
func WithSoftware(s string) Option
func WithDateTime(t time.Time) Option       // formatted as "YYYY:MM:DD HH:MM:SS" per TIFF 6.0
func WithSourceFormat(s string) Option      // emits TagWSISourceFormat on L0
func WithToolsVersion(s string) Option      // emits TagWSIToolsVersion on L0
```

These populate L0 IFD with standard TIFF tags 271 (Make), 272 (Model), 305 (Software), 306 (DateTime), plus the WSI tag block. All optional; absent values mean "don't emit the tag."

L0 ImageDescription for non-SVS-shaped outputs gets a wsi-tools provenance string built by the CLI:

```
wsi-tools/0.2.0 transcode source=philips-tiff codec=jxl mpp=0.25 mag=40x scanner="Hamamatsu C9600" date=2026-01-15
```

Doesn't start with "Aperio", so opentile-go won't mis-detect as SVS. Per-IFD ImageDescription on associated IFDs gets `wsi-imagetype=label` etc. — redundant with `WSIImageType` tag, but human-readable via `tiffinfo`.

### Component: `internal/codec` (5 new subpackages)

All 5 follow the v0.1 `codec.Encoder` interface (`LevelHeader`, `EncodeTile`, `TIFFCompressionTag`, `ExtraTIFFTags`, `Close`). Per-codec specifics:

#### `jpegli`

- **Library:** `libjxl` (which ships `libjpegli`). `pkg-config: libjxl_threads libjxl`.
- **API:** `<jpegli/encode.h>` — libjpeg-API-compatible. Reuse `internal/codec/jpeg`'s `wsi_encode` C helper structure verbatim, swap `jpeg_*` → `jpegli_*`. Same JCS_RGB storage, same APP14 marker, same abbreviated-mode tile output.
- **Output:** standard JPEG bitstream (TIFF Compression=7). Indistinguishable from `internal/codec/jpeg` to opentile-go; container = SVS-shaped when source is SVS, generic TIFF otherwise.
- **Quality:** `--quality 1..100` direct. Codec-opt: `jpegli.distance=N` (Butteraugli, alternative to quality).

#### `jpegxl`

- **Library:** `libjxl`. `pkg-config: libjxl_threads libjxl`.
- **API:** libjxl's `JxlEncoder` C API. One frame per tile via `JxlEncoderAddImageFrame` + `JxlEncoderFinish`.
- **Output:** JPEG-XL codestream (TIFF Compression=50002, Adobe-allocated draft).
- **Quality:** `--quality 1..100` mapped to `distance` parameter (`distance ≈ 100/quality - 1` on the per-codec calibration; documented in the codec subpackage). Codec-opts: `jxl.distance=N`, `jxl.effort=1..9`, `jxl.lossless=true`.
- **No JPEGTables-style shared header** — each tile is self-contained.

#### `avif`

- **Library:** `libavif`. `pkg-config: libavif`.
- **API:** `avifEncoder` per tile. Encode RGB via `avifRGBImageSetDefaults` + `avifImageRGBToYUV` (libavif handles the conversion internally) + `avifEncoderAddImage` + `avifEncoderFinish`.
- **Output:** AVIF still per tile (TIFF Compression=60001, wsi-tools-private — no standardised TIFF tag for AVIF).
- **Quality:** `--quality 1..100` → AVIF quantizer (lower quantizer = higher quality; libavif provides a quality→quantizer helper). Codec-opts: `avif.speed=0..10`, `avif.quality-alpha=N`.

#### `webp`

- **Library:** `libwebp`. `pkg-config: libwebp`.
- **API:** `WebPEncodeRGB` (lossy) or `WebPEncodeLosslessRGB` (lossless), one call per tile. Smallest cgo surface of the 5.
- **Output:** WebP per tile (TIFF Compression=50001, Adobe-allocated).
- **Quality:** `--quality 1..100` direct. Codec-opts: `webp.lossless=true`, `webp.method=0..6`.

#### `htj2k`

- **Library:** OpenJPH. **No `pkg-config` shipped on Homebrew yet** — explicit `#cgo CXXFLAGS: -I/opt/homebrew/include/openjph` and `#cgo LDFLAGS: -lopenjph`. Slightly more cgo plumbing than the others.
- **API:** OpenJPH's C++ API wrapped in a C shim (similar pattern to `internal/decoder/jpeg2000`'s OpenJPEG shim). One J2K codestream per tile (no JP2 boxing).
- **Output:** HTJ2K per tile (TIFF Compression=60003, wsi-tools-private — disambiguates from JP2K's 33003/33005).
- **Quality:** `--quality 1..100` → quality layer count and/or PSNR target; OpenJPH's quality model is JP2K-flavored. Codec-opts: `htj2k.qstep=N`, `htj2k.layers=N`.

#### Cross-cutting

- Each subpackage registers via `init() { codec.Register(Factory{}) }`.
- **Per-codec build tags**: each subpackage gates its `init()` registration behind a unique tag — `//go:build !nojpegli`, `//go:build !nojxl` (for `internal/codec/jpegxl/`), `//go:build !noavif`, `//go:build !nowebp`, `//go:build !nohtj2k`. Tags compose: `-tags nojpegli nojxl` disables both jpegli AND jpegxl, eliminating the libjxl link entirely. `-tags nojxl` alone disables only jpegxl; jpegli still pulls libjxl in.
- `internal/codec/all/all.go` blank-imports all 5 new codecs.
- Slim binaries skip the corresponding codec subpackages; `wsi-tools transcode --codec <skipped>` returns `ErrCodecUnavailable` and `wsi-tools doctor` reports the codec as unavailable.

## CLI surface

### `wsi-tools transcode`

```
USAGE:
    wsi-tools transcode [flags] <input>

FLAGS:
    -o, --output PATH         Output file path (required).
        --codec NAME          Target codec. Six values accepted at v0.2.0:
                                  jpeg     — re-encode standard JPEG at a different --quality
                                  jpegli   — smaller/better JPEG via libjxl's jpegli encoder
                                  jpegxl   — JPEG-XL codestream
                                  avif     — AVIF still per tile
                                  webp     — WebP per tile
                                  htj2k    — High-Throughput JPEG 2000 codestream
        --quality N           Codec-agnostic 1..100. Default: 85.
        --codec-opt KEY=VAL   Codec-specific tuning. Repeatable. Examples:
                                  --codec-opt jxl.distance=1.5
                                  --codec-opt avif.speed=4
                                  --codec-opt webp.lossless=true
        --container svs|tiff  Output container. Default: 'svs' when source is SVS AND codec is
                              jpeg or jpegli; 'tiff' otherwise. Forcing 'svs' on other codecs
                              writes the Aperio header but tile bytes will be in the chosen
                              codec — only useful for testing your viewer.
        --jobs N              Worker goroutines. Default: NumCPU.
        --bigtiff auto|on|off Default: auto (promote when predicted output > 2 GiB).
    -f, --force               Overwrite output if it exists.
        --quiet               Suppress progress bar.
        --verbose             Per-level summaries to stderr.
        --log-level LEVEL     debug|info|warn|error. Default: info.
        --log-format FORMAT   text|json. Default: text.

EXAMPLES:

    # Transcode a JPEG-encoded SVS to JPEG-XL inside a generic pyramidal TIFF.
    wsi-tools transcode --codec jpegxl -o slide-jxl.tiff slide.svs

    # jpegli on SVS — output is still a valid Aperio SVS, just smaller per-tile.
    wsi-tools transcode --codec jpegli -o slide-jpegli.svs slide.svs

    # AVIF with a faster encoder preset.
    wsi-tools transcode --codec avif --codec-opt avif.speed=6 -o out.tiff in.svs

    # Lossless WebP for archival.
    wsi-tools transcode --codec webp --codec-opt webp.lossless=true -o out.tiff in.svs
```

**Behaviour:**

- Source format gate: open via `internal/source.Open(input)`. If it returns `ErrUnsupportedFormat`, exit with a clear message naming the source format and pointing at the v0.2 sanity policy.
- Container resolution: per the policy above, with `--container` overriding.
- Output BigTIFF promotion: predicted size = sum across levels of `width × height × bytes_per_compressed_pixel_estimate`; auto when > 2 GiB or input was BigTIFF. Threshold per-codec inside the wrapper (lossy codecs estimate 0.5–1.5 bpp; lossless WebP estimates ~3 bpp).

### `wsi-tools doctor` (extended)

```
$ wsi-tools doctor

wsi-tools 0.2.0 — codec / library health check.

Codecs:
  ✓ jpeg      (libjpeg-turbo 3.0.4)
  ✓ jpegli    (libjxl 0.10.2)
  ✓ jpegxl    (libjxl 0.10.2)
  ✓ avif      (libavif 1.0.4)
  ✓ webp      (libwebp 1.4.0)
  ✓ htj2k     (openjph 0.16.0)

Source decoders:
  ✓ jpeg      (libjpeg-turbo 3.0.4)
  ✓ jpeg2000  (openjpeg 2.5.4)

Required: opentile-go v0.12.0 ✓
```

Each codec line shows the underlying library version where the library exposes it (libjxl, libavif, libwebp via runtime version macros; OpenJPH via header macro). Missing libs reported as `✗` with a one-line install hint.

## Error handling

Identical surface to v0.1 plus:

- **Up-front input validation**: `--codec` resolves; source-format sanity gate; required cgo lib for chosen codec is loaded.
- **`source.ErrUnsupportedFormat`**: clear message naming the source format and pointing at v0.2 policy. Exit code 2 (usage-style).
- **`source.ErrUnsupportedSourceCompression`**: emitted when an IFE source uses Iris-proprietary compression that v0.2 can't decode.
- **`codec.ErrCodecUnavailable`**: emitted when a build-tag-skipped codec is requested. Doctor reports the same.
- **Per-tile typed cgo errors**: `codec.ErrEncodeFailed{Codec, Reason}` — same as v0.1.
- **Atomic output + SIGINT cancellation**: same as v0.1.

## Observability

- Progress bar (`mpb/v8`) — total tiles across all levels, decremented as encode completes.
- Structured logs via `slog` — same shape as v0.1 (`--log-format text|json`, `--log-level`, `--verbose`).
- Final summary on stdout: input size, output size, ratio, total wall time, tiles encoded, peak RSS.

## Testing strategy

### Layer 1 — unit tests (race-detector, no fixtures)

- **`internal/source/`**: synthetic Tiler tests for sanity-gate accept/reject; pass-through behavior for the 6 sane formats.
- **`internal/codec/{jpegxl,jpegli,avif,webp,htj2k}/`**: per-codec round-trip — encode an RGB gradient, decode via the format's reference decoder, assert PSNR > codec-specific threshold (~38 dB at q=85; byte-exact for lossless modes). Each gated by `//go:build !no<codec>`.
- **`internal/wsiwriter/wsitags`**: round-trip — write IFDs with `WSIImageType` tags, re-read via the existing `golang.org/x/image/tiff` (tag readable as a generic ASCII tag), assert values match.

### Layer 2 — integration tests (`-tags integration`, gated by `WSI_TOOLS_TESTDIR`)

- **Per-codec sweep**: transcode `CMU-1-Small-Region.svs` to each of the 5 codecs. Re-open output via opentile-go; assert format, level count, Compression tag value, output tile readable. One subtest per codec.
- **Per-source-format sweep**: transcode each sane source format to JPEG-XL (representative codec — newest + most metadata churn) and assert structural output validity. Source fixtures from opentile-go's `sample_files/`:
  - SVS: `CMU-1-Small-Region.svs`
  - Philips-TIFF: `philips-tiff/Philips-1.tiff`
  - OME-TIFF (tiled): `ome-tiff/Leica-1.ome.tiff`
  - BIF: `bif/Ventana-1.bif`
  - IFE: `ife/cervix_2x_jpeg.iris`
  - Generic TIFF: `generictiff/CMU-1.tiff`
  - NDPI (rejection): `ndpi/CMU-1.ndpi` — assert `ErrUnsupportedFormat`.
- **Round-trip Kind() assertion**: for each output, assert opentile-go's re-read `AssociatedImage.Kind()` matches what we wrote — catches the v0.1 IFD-ordering bug + verifies `WSIImageType` tag correctness once opentile-go is taught to read tag 65080. Until opentile-go is updated, the assertion validates that opentile-go's heuristic + our IFD ordering agree.
- **4.8 GB BigTIFF re-included**: `svs_40x_bigtiff.svs` is added back to the integration sweep at v0.2.0. Streaming makes it tractable. RSS ceiling assertion (peak < 1 GiB) added to catch regressions.

### Layer 3 — viewer-compat checklist

`docs/viewer-compat.md` extended with a per-codec, per-viewer matrix. Manual verification — not in CI. Updated as each codec lands.

```
## v0.2 — transcode tool

| Codec   | QuPath | openslide-bin | Custom Viewer | OpenSeadragon |
|---------|--------|---------------|---------------|---------------|
| jpegli  | -      | -             | -             | -             |
| jpegxl  | -      | -             | -             | -             |
| avif    | -      | -             | -             | -             |
| webp    | -      | -             | -             | -             |
| htj2k   | -      | -             | -             | -             |
```

### Performance harness (optional, not gated in CI)

`tests/bench/transcode_bench_test.go` reports tiles/sec, total wall time, RSS peak per (codec, source-fixture) pair. v0.2 baseline numbers committed for regression spotting in v0.3.

## Build / packaging

- **`go.mod`**: bump `github.com/cornish/opentile-go` to `v0.12.x` (or `v0.11.0` once tagged).
- **macOS workflow** (`.github/workflows/ci.yml`):
  - `brew install jpeg-turbo openjpeg jpeg-xl libavif webp openjph pkg-config libtiff`.
  - Run `go build`, `go vet`, `go test ./... -race -count=1`.
- **Windows workflow**:
  - msys2 install: `mingw-w64-x86_64-libjxl mingw-w64-x86_64-libavif mingw-w64-x86_64-libwebp mingw-w64-x86_64-libjpeg-turbo mingw-w64-x86_64-openjpeg2 mingw-w64-x86_64-pkgconf` (and OpenJPH if available).
  - **Concrete risk: openjph is not in msys2 packaging as of 2026-05.** Fallback: build the Windows binary with `-tags nohtj2k` and document the gap. `wsi-tools doctor` reports htj2k as unavailable on the Windows binary; users who need it build from source with msys2 + a manually compiled OpenJPH.
- **Build tags** plumbed: per-codec `-tags no<codec>` skip the corresponding `init()` registration. `make build-slim` target as a smoke-test (default: skip nothing).
- **No pre-built binaries** at v0.2.0. `go install github.com/cornish/wsi-tools/cmd/wsi-tools@v0.2.0` is the supported install path. Linux untested but expected to work.

## Open questions for implementation time (not blockers for the spec)

- **Exact OpenJPH install path on macOS Homebrew.** As of 2026-05 the formula is `openjph` and installs headers under `/opt/homebrew/include/openjph/`. Verify at implementation time.
- **libjxl version in Homebrew.** As of 2026-05 the bottle is `jpeg-xl 0.10.x`. If 0.11+ has shipped, the jpegli C API surface may have changed — verify against `<jpegli/encode.h>` in the installed bottle.
- **AVIF tile alpha handling.** Pathology RGB tiles have no alpha, but libavif's API expects an `avifRGBImage` with explicit alpha-presence flag. Set `avifRGB.alpha = 0` and `avifRGB.format = AVIF_RGB_FORMAT_RGB`.
- **HTJ2K compressed-bytes wrapping.** OpenJPH outputs a J2K codestream; the TIFF Compression=60003 private value documents that. Verify our generic-TIFF reader can round-trip these bytes by re-decoding via OpenJPH.
- **`v0.1 downsample IFD ordering` fix scope.** Bundled into v0.2, but worth a separate commit so the fix is bisectable. Add a test asserting the corrected ordering before changing the writer.

## Future work (out of scope for v0.2.0)

- v0.2.1: add HEIF, JPEG-LS, JPEG-XR codecs (HEIF via libheif; JPEG-LS via CharLS; JPEG-XR via jxrlib source build). Also add `--re-encode-associated` flag if a use case surfaces.
- v0.2.2: add Basis Universal codec (basis_universal source build + KTX2 wrapping). Add jpeg + jpeg2000 as transcode targets (decoders already shipped at v0.1).
- v0.2.3: streaming retrofit for `downsample` — eliminate the full L0 raster materialisation by interleaving decode + resample + encode into a single streaming pass per output level.
- v0.2.x: Leica SCN source support — single-image single-channel SCNs first (the simplest subset), then multi-image (per-`Image` parallel pyramids) and multi-channel (`SizeC > 1` fluorescence) as separate steps.
- v0.3: NDPI + OME-OneFrame source support via virtual-tile materialisation; libjpeg `longjmp` error recovery; Linux CI; cross-codec round-trip tests.
- v0.4: GUI front-end; absorption of `cornish/wsi-label-tools`.

## References

- **opentile-go v0.12** — `github.com/cornish/opentile-go` (the read side; v0.11 introduces format-string disambiguation for Philips and OME variants).
- **opentile-go SVS classifier** — `formats/svs/series.go:classifyPages`. Authoritative on the IFD ordering rule used by SVS-shaped output.
- **opentile-go generic-TIFF classifier** — `formats/generictiff/classifier.go`. Heuristic for trailing IFD classification; `WSIImageType` tag 65080 is the future-explicit override path.
- **DICOM Whole Slide Imaging (PS3.3 Sup. 145)** — basis for the `WSIImageType` value vocabulary (DICOM uses VOLUME / LABEL / OVERVIEW / THUMBNAIL; we use lowercase + a few extensions for opentile-go's broader Kind set).
- **TIFF tag values:**
  - Standard: `libtiff` `tif_dir.h` (COMPRESSION_*, TIFFTAG_*).
  - AwareSystems TIFF tag reference: https://www.awaresystems.be/imaging/tiff/tifftags.html.
  - Adobe-allocated drafts: WebP (50001), JPEG-XL (50002).
  - wsi-tools-private (≥ 60001 / ≥ 65080): documented in `docs/tiff-tags.md`.
- **Codec library docs:**
  - libjxl: https://github.com/libjxl/libjxl
  - libavif: https://github.com/AOMediaCodec/libavif
  - libwebp: https://chromium.googlesource.com/webm/libwebp
  - OpenJPH: https://github.com/aous72/OpenJPH
