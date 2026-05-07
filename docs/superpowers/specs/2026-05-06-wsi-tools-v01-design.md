# wsi-tools v0.1 — design

**Status:** draft, awaiting user review
**Date:** 2026-05-06
**Author:** brainstormed with Claude Opus 4.7
**Module:** `github.com/cornish/wsi-tools`

## Goal

A Go-based "Swiss-army knife" of WSI (whole-slide imaging) utilities. v0.1 ships two CLIs needed immediately for downstream work:

1. **`wsi-tools downsample`** — produce a lower-magnification copy of a WSI by downsampling level 0 (e.g. 40x SVS → 20x SVS), regenerating the entire pyramid from the new base, and copying associated images (label, macro, thumbnail, overview) verbatim. Format-preserving (SVS in → SVS out).
2. **`wsi-tools transcode`** — re-encode the pyramid tiles in a different compression codec while keeping the rest of the file unchanged. Targets eleven codecs at v1: `jpeg`, `jpegli`, `jpegxl`, `jpeg2000`, `htj2k`, `jpegls`, `jpegxr`, `avif`, `heif`, `webp`, `basis`. The nine JPEG-alternative codecs (everything except `jpeg` and `jpeg2000`) are the user-named targets; `jpeg` and `jpeg2000` round out the set since their cgo libs are required on the source-decode side anyway. Primary motivation: feeding test fixtures into a viewer that already supports these codecs but for which no public WSI fixtures exist.

v0.1 is SVS-only on the input side. Generic pyramidal TIFF output supported by the transcode tool when the chosen codec can't fit in the SVS convention. NDPI, Philips, OME, BIF, IFE source support is deferred to v0.2+.

A future GUI front-end and the absorption of `cornish/wsi-label-tools` are explicitly out of scope here but architecturally accommodated.

## Non-goals (v0.1)

- Cross-platform CI (macOS dev target only; Linux likely works, no lane).
- Windows.
- Tracing / Prometheus metrics / structured progress feeds for an external GUI.
- Cross-codec / cross-fixture combinatorial test sweeps. One canonical fixture per codec.
- Source slides in formats other than SVS.
- A general-purpose libtiff replacement (the writer is intentionally narrow).

## Constraints / decisions made during brainstorming

- **Reader = `github.com/cornish/opentile-go`** as a Go module dep. Not forked; we use it as-is.
- **Writer = internal package `internal/wsiwriter`**. Pure Go for the TIFF/SVS structural bookkeeping; cgo only inside codec wrappers. Extract to its own module later if a third caller appears.
- **Pyramid policy on downsample = regenerate the whole pyramid from new L0.** Each output level is computed from the previous output level (not from source levels). Output level count and ratio chain mirror the source's.
- **Downsample algorithm = libjpeg-turbo `1/2`/`1/4`/`1/8` in-decode scale on JPEG sources, 2×2 area average on full-resolution rasters when fast-scale isn't applicable, Lanczos via libvips for non-power-of-2 factors.**
- **Associated images (label, macro, thumbnail, overview) = pass-through verbatim** in both subcommands. Bytes are copied directly from the source IFD to the output IFD; no decode/encode.
- **CLI shape = single binary, cobra subcommands.** `wsi-tools downsample`, `wsi-tools transcode`, `wsi-tools doctor`, `wsi-tools version`. Strong `--help` from day one.
- **Compression-tag policy = pragmatic.** Use the closest existing TIFF Compression value where one exists (jpegli=7, JPEG-LS=34712, JPEG-XL=50002 draft, WebP=50001 Adobe-allocated, HTJ2K via JP2 codestream reusing 33005 or a private code, JPEG-XR via Microsoft's allocated values). Invent private codes (≥32768) for AVIF, HEIF, Basis Universal. Document the full mapping in `docs/compression-tags.md`. Exact tag numbers verified against libtiff at implementation time, not asserted from memory.
- **JPEG-on-SVS writer must produce abbreviated tiles + per-level shared `JPEGTables`.** Mirrors what opentile-go reads; mismatch breaks every Aperio-aware viewer.
- **Aperio APP14 marker.** SVS JPEG tiles carry an Adobe-style APP14 marker declaring RGB-not-YCbCr colourspace; opentile-go has the canonical bytes in `internal/jpeg.adobeAPP14`. Writer mirrors that constant.
- **Atomic output.** Every write goes to `<output>.tmp` and renames on success; partial outputs are never visible.
- **Quality flag.** Single `--quality 1..100` knob across codecs (mapped per-codec inside the wrapper); `--codec-opt key=value` repeatable for codec-specific tuning.
- **BigTIFF.** Auto-promote to BigTIFF when the source was BigTIFF or predicted output >2 GB; `--bigtiff auto|on|off`.

## Repo layout

```
wsi-tools/
├── cmd/wsi-tools/                # cobra root + subcommands
│   ├── main.go
│   ├── downsample.go
│   ├── transcode.go
│   ├── doctor.go
│   └── version.go
├── internal/
│   ├── wsiwriter/                # SVS / TIFF writer
│   │   ├── tiff.go               # IFD layout, TileOffsets/TileByteCounts, BigTIFF
│   │   ├── svs.go                # Aperio ImageDescription rewriter, label/macro IFD shape
│   │   ├── compression.go        # TIFF Compression tag constants (incl. private codes)
│   │   └── jpegtables.go         # tables-only JPEG synthesis for shared-tables levels
│   ├── codec/                    # encode-side codec wrappers (per-codec subpackage)
│   │   ├── codec.go              # Encoder / EncoderFactory / registration
│   │   ├── jpeg/                 # libjpeg-turbo
│   │   ├── jpegli/               # libjxl/jpegli
│   │   ├── jpegxl/               # libjxl
│   │   ├── jpeg2000/             # OpenJPEG
│   │   ├── htj2k/                # OpenJPH
│   │   ├── jpegls/               # CharLS
│   │   ├── jpegxr/               # jxrlib
│   │   ├── avif/                 # libavif
│   │   ├── heif/                 # libheif
│   │   ├── webp/                 # libwebp
│   │   ├── basis/                # basis_universal + KTX2
│   │   └── all/                  # umbrella registration package
│   ├── decoder/                  # decode-side wrappers (smaller surface)
│   │   ├── jpeg.go               # libjpeg-turbo, supports tjDecompress2 1/N fast scale
│   │   └── jpeg2000.go           # OpenJPEG (Aperio JP2K source slides)
│   ├── resample/
│   │   ├── area.go               # 2×2 area average (pure Go)
│   │   └── lanczos.go            # libvips Lanczos fallback
│   └── pipeline/
│       └── pipeline.go           # decode → process → encode worker plumbing
├── docs/
│   ├── compression-tags.md       # canonical codec → TIFF Compression value table
│   ├── viewer-compat.md          # manual viewer-compatibility checklist
│   └── superpowers/
│       ├── specs/                # this file lives here
│       └── plans/                # implementation plans
├── tests/
│   ├── integration/              # WSI_TOOLS_TESTDIR-gated integration tests
│   └── bench/                    # perf harness, not in CI
├── sample_files/                 # gitignored; user soft-links to opentile-go's pool
├── Makefile
├── README.md
├── CLAUDE.md
├── LICENSE                       # Apache 2.0
└── go.mod
```

## Architecture

### Data flow

Both subcommands share the skeleton:

```
opentile-go ──► decode ──► process ──► encode ──► wsiwriter
  (source        (per      (downsample:  (target     (target
   slide)         tile)     2× area;      codec)      slide)
                            transcode:
                            no-op)
```

#### Per-tile lifecycle

1. **Source read.** `opentile-go` `Level.TileInto(x, y, dst)` fills a pooled buffer with raw compressed bytes. Mmap-backed in v0.9+; ≈150 ns/op.
2. **Decode.** Dispatch on source compression tag:
   - JPEG (7) → libjpeg-turbo. For downsample, `tjDecompress2` with `scaleNum/scaleDen = 1/2|1/4|1/8` so the decoded raster is already at target resolution. For transcode, full-resolution decode.
   - JPEG 2000 (33003 / 33005) → OpenJPEG. Always full-resolution decode.
3. **Process.** For downsample: if source codec didn't natively scale (non-JPEG sources, or a non-power-of-2 factor), run a 2×2 area average; for non-power-of-2 factors fall back to libvips Lanczos. For transcode: pixels untouched.
4. **Encode.** Codec wrapper produces compressed bytes for the target compression.
5. **Sink.** `wsiwriter` appends the encoded bytes and updates the output IFD's TileOffsets/TileByteCounts arrays.

#### Per-pyramid-level lifecycle

- **Downsample.** Output L0 is built from source L0. Each subsequent output level (L1, L2, …) is built from the freshly-written previous output level — never from source L1+ — to avoid compounding generation loss across pyramid levels.
- **Transcode.** Each source level transcodes one-to-one to the matching output level. No cross-level dependencies; levels can run concurrently if useful.

#### Concurrency

- A worker pool of N decode→process→encode goroutines (default `runtime.NumCPU()`) feeds a single writer goroutine that owns the output file. TIFF tile-table assembly is inherently sequential; the writer goroutine is the only writer.
- Per-tile buffers come from a `sync.Pool` sized to opentile-go's `Level.TileMaxSize()`. Steady-state allocations are zero per tile on the hot path.
- `--jobs N` overrides the default worker count.

### Component: `internal/wsiwriter`

The load-bearing piece — opentile-go reads but doesn't write. Designed narrow on purpose; not a libtiff replacement.

#### Public surface

```go
type Writer struct{ /* ... */ }

func Create(path string, opts ...Option) (*Writer, error)
//   opts: WithBigTIFF(bool|"auto"), WithByteOrder(binary.ByteOrder),
//         WithSVSImageDescription(string)

type LevelSpec struct {
    ImageWidth, ImageHeight   uint32
    TileWidth, TileHeight     uint32
    Compression               uint16   // TIFF tag 259
    PhotometricInterpretation uint16   // typically YCbCr for JPEG, RGB for JXL/AVIF/etc.
    JPEGTables                []byte   // tables-only JPEG (SOI+DQT+DHT+EOI), per-level
    JPEGAbbreviatedTiles      bool     // tiles omit DQT/DHT when true
    ICCProfile                []byte   // optional, externalised when codec doesn't carry it
    ExtraTags                 []TIFFTag
}
func (w *Writer) AddLevel(LevelSpec) (*LevelHandle, error)
func (h *LevelHandle) WriteTile(x, y uint32, compressed []byte) error

type AssociatedSpec struct {
    Kind           string  // "label", "macro", "thumbnail", "overview"
    Compressed     []byte
    Width, Height  uint32
    Compression    uint16
    NewSubfileType uint32  // Aperio convention: label=1, macro=9, etc.
    ExtraTags      []TIFFTag
}
func (w *Writer) AddAssociated(AssociatedSpec) error

func (w *Writer) Close() error
```

#### Backing implementation

- **Hand-rolled pure-Go TIFF writer.** TIFF's structure is well-bounded for the surface we need (tiled IFDs, JPEGTables, BigTIFF offset widening, Aperio-style ImageDescription). Keeps cgo confined to `internal/codec/*`.
- **Not `golang.org/x/image/tiff`.** No tiled writes, no JPEG-in-TIFF, no custom Compression values. Wrong shape.
- **Not libtiff via cgo.** Adds another link surface, and libtiff's "OJPEG vs new-style JPEG" history is a known minefield. Reserved as a fallback build-tag backend if a viewer-compat issue surfaces that we can't resolve in pure Go.
- **BigTIFF.** Auto-promote when input was BigTIFF or predicted output >2 GB; explicit `--bigtiff` flag overrides.

#### SVS-specific bits

- L0 `ImageDescription` is the Aperio header: `Aperio Image Library v…\r\n<W>x<H> [...] |MPP = <new> |AppMag = <new> |…`. Writer parses the source `ImageDescription`, mutates `MPP` (×factor) and `AppMag` (÷factor) for downsample, preserves all other key=value pairs.
- Subsequent pyramid IFDs use Aperio's stripped per-level `ImageDescription` form. Match what opentile-go reads on the input side.
- Associated-image IFD shape (NewSubfileType, ImageWidth/Length, Compression, StripOffsets vs single strip): pinned against a real fixture during implementation.

#### JPEG-on-SVS encoding (correctness trap)

Aperio SVS stores JPEG tiles in **abbreviated** form — tile bytes carry no DQT/DHT, the level's IFD carries a tables-only JPEG in tag 347 (`JPEGTables`). The writer must:

1. Precompute a shared quant + Huffman table set per level (use libjpeg-turbo's standard tables for the chosen quality, or extract DQT/DHT from one throwaway encoded tile).
2. Encode every tile abbreviated (`jpeg_suppress_tables(cinfo, TRUE)`) against those shared tables. Tile bytes = `SOI + APP14 (Aperio RGB marker) + SOS + entropy + EOI`.
3. Emit the table set once into the IFD's tag 347 as `SOI + DQT + DHT + EOI`.

The codec-wrapper API surfaces this as `(jpegTables []byte, encoder TileEncoder)` so the caller threads `jpegTables` into `LevelSpec.JPEGTables` and `LevelSpec.JPEGAbbreviatedTiles = true`. For non-JPEG target codecs, `JPEGTables` is empty and `JPEGAbbreviatedTiles` is false.

### Component: `internal/codec`

Eleven codec targets at v1 (nine user-named JPEG alternatives + `jpeg` + `jpeg2000`). Each codec is its own subpackage; uniform interface so the pipeline doesn't grow an 11-arm switch.

```go
type Encoder interface {
    LevelHeader() []byte                              // per-level shared bytes (e.g., JPEGTables)
    EncodeTile(rgb []byte, w, h int, dst []byte) ([]byte, error)
    TIFFCompressionTag() uint16
    ExtraTIFFTags() []wsiwriter.TIFFTag
    Close() error
}

type EncoderFactory interface {
    Name() string
    NewEncoder(LevelGeometry, Quality) (Encoder, error)
}

type LevelGeometry struct {
    TileWidth, TileHeight int
    PixelFormat           PixelFormat   // RGB8, RGBA8, YCbCr420, ...
    ColorSpace            ColorSpace    // sRGB, DisplayP3, ICC{[]byte}, ...
}

type Quality struct {
    Knobs map[string]string  // codec-specific; wrapper interprets
}

type Decoder interface {
    DecodeTile(compressed []byte, dst []byte, scaleNum, scaleDen int) ([]byte, error)
}
```

**Registration** mirrors opentile-go's format-factory pattern: each codec subpackage registers via `init()`, the umbrella `internal/codec/all` import pulls them all in, `codec.Lookup("jpegxl")` returns the factory.

**Per-codec build tags** (default build = all codecs): `-tags noavif noheif` etc. for slimmed binaries. `wsi-tools doctor` reports which codec libraries are linked + loadable at runtime.

**Quality knobs:** single `--quality 1..100` flag mapped per-codec; `--codec-opt jxl.distance=1.5` etc. for tuning. Per-codec quality semantics documented in each subpackage's docstring.

### Component: `internal/decoder`

Smaller than `internal/codec` because the source-side codec set is bounded by what SVS uses:

- **JPEG** via libjpeg-turbo. Supports `tjDecompress2` with `scaleNum/scaleDen` ∈ {1/1, 1/2, 1/4, 1/8} for the fast in-decode scale path.
- **JPEG 2000** via OpenJPEG. Full-resolution decode only.

When v0.2 expands source-format coverage, this package grows. v0.1 is intentionally minimal.

### Component: `internal/resample`

- `area.go`: 2×2 area average (pure Go), called when fast-scale-decode wasn't possible (e.g., JP2K source) but the factor is still a power of 2.
- `lanczos.go`: thin libvips wrapper, plumbed but **not exercised at v0.1** — v0.1 rejects non-power-of-2 factors at the CLI layer. Lanczos ships behind a `nolanczos` build tag stub that returns `ErrNotImplemented`; the real wrapper lands in v0.2 when arbitrary-factor downsampling becomes a use case.

### Component: `internal/pipeline`

Worker pool plumbing: tile-coordinate producer → N worker goroutines (decode/process/encode) → single sink goroutine (writer). Cancellation via `context.Context`. Shared `sync.Pool` of byte buffers sized to `Level.TileMaxSize()`. First per-tile error cancels the context; sink finalises by removing the `.tmp` file.

## CLI surface

### `wsi-tools downsample`

```
USAGE:
    wsi-tools downsample [flags] <input>

FLAGS:
    -o, --output PATH         Output file path (required).
        --factor N            Integer downsample factor; must be a power of 2.
                              Default: 2 (40x → 20x).
        --target-mag N        Alternative to --factor: derive factor from the
                              source's AppMag (e.g. --target-mag 20 on a 40x = factor 2).
        --quality N           JPEG / target-codec quality, 1..100. Default: 90.
        --jobs N              Worker goroutines. Default: NumCPU.
        --bigtiff auto|on|off Output BigTIFF mode. Default: auto.
        --resampler area|lanczos
                              Resampling algorithm. v0.1 supports 'area' only;
                              'lanczos' returns ErrNotImplemented and is reserved
                              for v0.2 when arbitrary-factor downsampling lands.
    -f, --force               Overwrite output if it exists.
        --quiet               Suppress progress bar.
        --verbose             Per-level summaries to stderr.
        --log-level LEVEL     debug|info|warn|error. Default: info.
        --log-format FORMAT   text|json. Default: text.
```

**Behaviour:**

- `--factor` and `--target-mag` are mutually exclusive; if neither is set, default = `--factor 2`.
- v0.1 rejects any factor that isn't a power of 2 (∈ {2, 4, 8, 16}). `--target-mag` derives the factor from `source AppMag ÷ target-mag` and errors out if the result isn't an integer power of 2.
- `--target-mag` requires the source's AppMag to be parseable from `ImageDescription`; errors out with a clear message if not.
- Output level count = source level count; output ratio chain mirrors source's.
- Output `MPP` = source `MPP × factor`; output `AppMag` = source `AppMag ÷ factor`.

### `wsi-tools transcode`

```
USAGE:
    wsi-tools transcode [flags] <input>

FLAGS:
    -o, --output PATH         Output file path (required).
        --codec NAME          Target codec. One of:
                                  jpeg, jpegli, jpegxl, jpeg2000, htj2k,
                                  jpegls, jpegxr, avif, heif, webp, basis
        --quality N           Codec-agnostic 1..100. Default: 85.
        --codec-opt KEY=VAL   Codec-specific tuning. Repeatable.
        --container svs|tiff  Output container. Default: same as input where the
                              codec fits Aperio convention; else 'tiff'.
        --jobs N              Worker goroutines. Default: NumCPU.
        --bigtiff auto|on|off Default: auto.
    -f, --force               Overwrite output if it exists.
        --quiet               Suppress progress bar.
        --verbose             Per-level summaries to stderr.
        --log-level LEVEL     debug|info|warn|error. Default: info.
        --log-format FORMAT   text|json. Default: text.
```

**Behaviour:**

- Container defaults: `--codec jpeg|jpegli` keeps SVS shape (Aperio convention supports it). All other codecs default to `--container tiff` (generic pyramidal TIFF), since SVS's `ImageDescription` and viewer ecosystem don't expect them. `--container svs` can be forced for any codec; we'll write the Aperio header but tile bytes will be in the chosen codec — useful only for testing your viewer.
- `--codec-opt` keys are namespaced by codec (`jxl.distance`, `avif.speed`, `webp.lossless`, etc.). Unknown keys error early.

### `wsi-tools doctor`

Reports installed codec libraries with versions (where the lib exposes them) and a one-line install hint for missing optional ones. Required deps (libjpeg-turbo, opentile-go) report `✓` / `✗`; optional codecs report `✓` / `✗ <hint>`. Exits non-zero if any required dep is missing.

### `wsi-tools version`

Prints version, git SHA, Go version, and which codec build tags were active.

## Error handling

- **Up-front input validation**: paths exist / are writable; output absent unless `--force`; `--factor` is a power of 2; codec resolves; required cgo lib present; quality in range.
- **Atomic output**: `<output>.tmp` → `rename(2)` on success; `os.Remove(tmp)` on error.
- **Per-tile errors are fatal to the job.** Wrap with tile coordinate + level for traceability. No skip-and-continue — pathology data integrity bar too high.
- **Typed cgo errors**: `codec.ErrEncodeFailed{Codec, Reason}` / `codec.ErrDecodeFailed{...}` for attribution.
- **Signal handling**: SIGINT / SIGTERM cancel via `context.Context`, drain in-flight encodes, remove `.tmp`, exit 130.

## Observability

- **Progress bar** on stdout when stdout is a TTY (`vbauerster/mpb` or similar). Suppressed when piped or `--quiet`.
- **Structured logs** on stderr via `log/slog`. `--log-format text|json`, `--log-level debug|info|warn|error`.
- **Final summary** on stdout: input size, output size, ratio, total wall time, tiles encoded, peak RSS.
- **Not in v1**: tracing, Prometheus, structured progress feeds for a GUI. Add when GUI lands.

## Testing strategy

### Layer 1: unit tests (pure Go where possible)

- `internal/wsiwriter`: round-trip a synthetic 4-level pyramid + 3 associated images; re-open with opentile-go; assert dimensions, MPP, AppMag, tile counts, JPEGTables presence, byte-exact tile bytes.
- `internal/codec/<codec>`: each wrapper round-trips a known RGB image; encode → decode → PSNR / byte-exact-for-lossless. Gated by `//go:build <codec>` so tests skip when the cgo lib isn't installed.
- `internal/resample/area`: hand-computed expected output for tiny inputs.
- `internal/pipeline`: synthetic source/sink, error propagation, atomic-rename verification.

### Layer 2: integration tests (real fixtures via `WSI_TOOLS_TESTDIR`)

- Source slides come from opentile-go's existing pool via `WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files` (soft-link or env-var override). `sample_files/` in this repo is gitignored.
- `tests/integration/downsample_test.go`: for each SVS fixture, downsample 40x→20x, re-open with opentile-go, assert level count, dimensions (source/2 at L0 then source ratio chain below), MPP×2, AppMag÷2, all 3 associated images present with byte-exact bytes from source. Spot-check decoded tiles vs hand-computed area-average (PSNR > 45 dB).
- `tests/integration/transcode_test.go`: for each codec, transcode `CMU-1.svs` to that codec, re-open output, decode L0 tile (0,0), compare to source decoded tile (PSNR > codec-specific threshold). Verify Compression tag value matches `docs/compression-tags.md`.

### Layer 3: viewer-compat sanity (manual, documented, not in CI)

- `docs/viewer-compat.md`: checklist of (codec, viewer) pairs verified by eye. Records what we've confirmed actually loads, since the whole point of the transcode tool is to feed your viewer.
- Specifically for `jpegli` SVS: open in QuPath, openslide-bin, OpenSeadragon-via-DZI to confirm "still standard JPEG" works end-to-end with no viewer changes.

### Performance harness

- `tests/bench/`: `BenchmarkDownsample40to20` on `CMU-1.svs` reporting tiles/sec, total wall time, RSS peak. Not gated in CI; harness exists for hand-spotting regressions.

### Explicit non-tested-at-v1

- Cross-platform.
- Combinatorial codec × fixture sweeps.
- Codecs without macOS Homebrew formulas (jpegxr, basis) ship with `ErrCodecUnavailable` stubs until source-build path is stood up. `wsi-tools doctor` reports honestly.

## Build / packaging

- **Module path**: `github.com/cornish/wsi-tools`.
- **Go**: 1.23+ (matches opentile-go floor).
- **cgo deps documented in README.md**:
  - libjpeg-turbo (required) — `brew install jpeg-turbo` / `apt install libturbojpeg0-dev`
  - libjxl (jpegli + jpegxl) — `brew install jpeg-xl` / `apt install libjxl-dev`
  - libavif — `brew install libavif` / `apt install libavif-dev`
  - libheif — `brew install libheif` / `apt install libheif-dev`
  - libwebp — `brew install webp` / `apt install libwebp-dev`
  - openjp2 — `brew install openjpeg` / `apt install libopenjp2-7-dev`
  - openjph — `brew install openjph` (apt: source build)
  - charls — `brew install charls` (apt: source build)
  - jxrlib — source build (no formula on macOS or apt)
  - basis_universal — source build (no formula on macOS or apt)
  - libvips — `brew install vips` / `apt install libvips-dev`
- **`pkg-config`** resolves all of them at build time.
- **Build tags** for codec subsets. Default tags include all 11 target codecs.
- **Makefile**: `test`, `vet`, `cover`, `bench`, `install`.
- **Install**: `go install github.com/cornish/wsi-tools/cmd/wsi-tools@latest`.
- **Homebrew formula**: deferred to v0.2 once tagged.
- **License**: Apache 2.0 (matches opentile-go).
- **Versioning**: SemVer. v0.1 = downsample SVS 40x→20x + transcode SVS to one codec end-to-end correctly.

## Open questions for implementation time (not blockers for the spec)

- Exact TIFF Compression tag values for each codec (verify against libtiff's tag table; document in `docs/compression-tags.md`).
- Exact Aperio NewSubfileType values for label / macro / thumbnail / overview (pin against a real SVS fixture; opentile-go's reader is canonical).
- Whether to compute per-level standard JPEG tables (libjpeg-turbo defaults) or extract them from a throwaway encoded tile. Pick whichever produces cleaner abbreviated tiles when round-tripped through opentile-go.
- Default `--quality` per codec. 90 for libjpeg-turbo / jpegli is the common "transparent" point; codec-specific defaults (e.g., AVIF speed 6) tuned during integration tests.
- Whether to expose a `--strip-icc` flag for codecs that embed colour profiles. Probably yes; defer until first real use case.

## Future work (out of scope for v0.1)

- v0.2: NDPI + Philips + OME source support (downsample + transcode); codec source-build path for jpegxr / basis_universal; Linux CI lane.
- v0.3: BIF + IFE source support; absorb `cornish/wsi-label-tools` (`labels remove`, `labels replace`).
- v0.4+: GUI front-end shell (probably wails or fyne); structured progress feed for the GUI to consume.

## References

- **opentile-go** — `github.com/cornish/opentile-go` v0.10+ (this consumer's read side).
- **opentile-go SVS doc** — `docs/formats/svs.md` in opentile-go.
- **opentile-go perf doc** — `docs/perf.md` in opentile-go.
- **Aperio APP14 marker bytes** — `internal/jpeg.adobeAPP14` in opentile-go.
- **TIFF / BigTIFF** — Adobe TIFF 6.0 spec; BigTIFF community spec at awaresystems.be.
- **Compression tag values** — libtiff source `tif_dir.h`, AwareSystems TIFF tag reference.
- **JPEG-XL TIFF tag** — Adobe-allocated 50002 (draft).
- **JPEG-LS TIFF tag** — ISO-allocated 34712.
- **WebP TIFF tag** — Adobe-allocated 50001.
