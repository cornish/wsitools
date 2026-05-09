# wsi-tools v0.3 — opentile-go v0.14 migration + TileInto adoption

**Status:** Sealed for implementation
**Date:** 2026-05-08
**Predecessor:** [v0.2 transcode](2026-05-07-wsi-tools-v02-design.md)

## Goal

Bump the opentile-go dependency from v0.12.0 → v0.14.0, claim the new
capabilities that bump unlocks (novel-codec decoder recognition; richer
metadata round-trip), and migrate the per-tile read path to opentile-go's
allocation-free `TileInto` API. v0.13 and v0.14 are both purely additive
upstream — there are no breaking changes to absorb — so the migration is a
forward-looking upgrade rather than a defensive bump.

The headline user-visible win: integration tests can now round-trip our
transcoded outputs *through opentile-go itself* (instead of falling back to
external `tiffinfo` for structural validation), and the per-tile read path
on the streaming pipeline avoids one allocation per tile.

## Non-goals

- **Splice-prefix optimisation.** v0.13's `TilePrefix` / `TileBodyInto` /
  `SpliceJPEGTile` are bandwidth-deduplication helpers for client-server
  byte-passthrough scenarios. Our transcode pipeline decodes every tile, so
  it needs the complete spliced JPEG. `TileInto` is the right hot path for
  us; the splice family stays unused.
- **Streaming retrofit for `downsample`.** Still scheduled for a later v0.3.x;
  the v0.3.0 buffer-reuse work for downsample is in-place tile reuse only,
  not the full streaming pyramid build.
- **New codecs.** No jpegli, HEIF, JPEG-LS, JPEG-XR, Basis Universal,
  jpeg2000-as-encode-target. Those are independent items.

## Context — what v0.13 and v0.14 added upstream

### opentile-go v0.13 (additive)

Three new methods on `opentile.Level`:

- `TilePrefix() []byte` — constant per-level JPEG splice prefix; nil when no
  shared JPEGTables apply.
- `TileBodyInto(x, y, dst) (int, error)` — on-disk tile bytes without splice.
- `TileBodyMaxSize() int` — upper bound on `TileBodyInto` output size.

Plus `opentile.SpliceJPEGTile(prefix, body) ([]byte, error)` and
`opentile.ErrBadJPEGSplice`.

None of these affect existing callers; `Tile()` and `TileInto()` are
unchanged and remain the canonical decode-this-tile-now path.

### opentile-go v0.14 (additive)

- Three new `Compression` enum values: `CompressionWebP`,
  `CompressionJPEGXL`, `CompressionHTJ2K`. AVIF reuses the existing
  `CompressionAVIF` constant added in v0.8.
- Five new TIFF compression tag mappings in `formats/generictiff/tiled.go`:
  34712 (registered JP2K), 50001 (WebP), 50002 (JPEG XL), 60001 (AVIF),
  60003 (HTJ2K).
- Validator whitelist accepts the same five tag values.
- A wsi-tools-flavoured `ImageDescription` parser
  (`formats/generictiff/wsitools.go`) that recognises the
  `wsi-tools/<version>` ASCII envelope our v0.2 writer emits and populates
  `Metadata.Magnification` / `ScannerManufacturer` / `AcquisitionDateTime` /
  `MicronsPerPixel` from it. Lenient on missing or malformed values;
  forward-compatible with future wsi-tools fields.

The opentile-go contract for novel codecs is **byte-passthrough**: the
reader reports `Compression()` correctly but does not decode tiles.
Consumers bring their own libwebp / libjxl / libavif / OpenJPH. That's
fine for our round-trip integration test — we only need the reader to
parse the structure and recognise the compression tag.

## Current shape (relevant pieces)

`internal/source/source.go::Level` exposes:

```go
type Level interface {
    Index() int
    Size() image.Point
    TileSize() image.Point
    Grid() image.Point
    Compression() Compression
    Tile(x, y int) ([]byte, error) // returns a fresh allocation per tile
}
```

Three callsites:

- `internal/source/opentile.go:119` — adapter wrapping `opentile.Level.Tile`.
- `cmd/wsi-tools/transcode.go:269` — producer goroutine in the streaming
  worker pool. Allocates per tile.
- `cmd/wsi-tools/downsample.go:521` — sequential pyramid-build loop.
  Allocates per tile.

`internal/source/source.go::Compression` covers JPEG, JPEG2000, LZW, Deflate,
None, AVIF, IrisProprietary. Missing: WebP, JPEGXL, HTJ2K.

`internal/pipeline/pipeline.go::Tile` carries `Bytes []byte` and nothing
else — no per-tile lifecycle hook.

## Design

### Architecture

Three packages touched, no new packages, no rearrangement:

| Layer | Change |
|---|---|
| `go.mod` | bump opentile-go v0.12.0 → v0.14.0 |
| `internal/source/source.go` | extend `Level` (TileInto + TileMaxSize, drop Tile); extend `Compression` enum + `String()` |
| `internal/source/opentile.go` | adapter delegates to opentile-go's `TileInto` / `TileMaxSize`; map three new compression values |
| `internal/pipeline/pipeline.go` | add `Release func()` field to `Tile` for buffer-pool plumbing |
| `cmd/wsi-tools/transcode.go` | producer adopts `TileInto` + per-level `sync.Pool`; process worker calls `Release()` after decode |
| `cmd/wsi-tools/downsample.go` | sequential loop reuses one tile-sized buffer across iterations |
| `tests/integration/transcode_test.go` | replace tiffinfo-shell-out with opentile-go round-trip for the 4 novel codecs |
| `CHANGELOG.md` | add `[0.3.0]`; drop "opentile-go novel-codec decoders" from v0.2.0 deferred list |

### `source.Level` interface

Replace `Tile()` with `TileInto()` + `TileMaxSize()`. The
adapter delegates straight through:

```go
type Level interface {
    Index() int
    Size() image.Point
    TileSize() image.Point
    Grid() image.Point
    Compression() Compression
    TileMaxSize() int                            // NEW — pool sizing hint
    TileInto(x, y int, dst []byte) (int, error)  // NEW — replaces Tile
}
```

Adapter:

```go
func (l *opentileLevel) TileMaxSize() int { return l.lvl.TileMaxSize() }
func (l *opentileLevel) TileInto(x, y int, dst []byte) (int, error) {
    return l.lvl.TileInto(x, y, dst)
}
```

`Tile()` is removed entirely. Callers must allocate (or pool) their own
buffer of `TileMaxSize()` and use `TileInto`. This is one extra line at any
remaining one-shot callsite, and forces the buffer-reuse discipline at
hot-path callsites.

### Compression enum extension

`internal/source/source.go`:

```go
const (
    CompressionUnknown Compression = iota
    CompressionJPEG
    CompressionJPEG2000
    CompressionLZW
    CompressionDeflate
    CompressionNone
    CompressionAVIF
    CompressionIrisProprietary
    CompressionWebP    // NEW
    CompressionJPEGXL  // NEW
    CompressionHTJ2K   // NEW
)
```

`String()` gets three new branches: `"webp"`, `"jpegxl"`, `"htj2k"`.

`mapOpentileCompression` in `internal/source/opentile.go` gets three new
cases mapping `opentile.CompressionWebP` / `CompressionJPEGXL` /
`CompressionHTJ2K` to the new constants.

### Pipeline buffer-pool plumbing

The streaming pipeline currently treats every `pipeline.Tile` as
self-contained: producer allocates `Bytes`, worker mutates `Bytes`, sink
writes `Bytes`. With pooling, the producer's `Bytes` allocation comes from
a pool, and after the worker's decode step, the producer-allocated buffer
is no longer needed (the encode step produces a fresh codec-owned slice).

Adding lifecycle to `pipeline.Tile`:

```go
type Tile struct {
    Level   int
    X, Y    uint32
    Bytes   []byte
    Release func() // optional; called by Process after decode is done with Bytes
}
```

`Release` is nil-safe: existing producers (downsample, in-the-future others)
that don't pool can leave it nil. The transcode producer sets it. The
process closure invokes it once, between decode and encode.

The pipeline package itself stays opaque to the pool — `Release` is just an
optional field on the value flowing through the pipe. This keeps the
pipeline reusable for future non-pooled use cases.

### Transcode producer + worker

In `cmd/wsi-tools/transcode.go::transcodeOneLevel`:

```go
maxTileBytes := lvl.TileMaxSize()
pool := &sync.Pool{
    New: func() any {
        b := make([]byte, maxTileBytes)
        return &b
    },
}
// ...
Source: func(ctx context.Context, emit func(pipeline.Tile) error) error {
    for ty := 0; ty < grid.Y; ty++ {
        for tx := 0; tx < grid.X; tx++ {
            bufp := pool.Get().(*[]byte)
            n, err := lvl.TileInto(tx, ty, *bufp)
            if err != nil {
                pool.Put(bufp)
                return err
            }
            t := pipeline.Tile{
                Level:   lvl.Index(),
                X:       uint32(tx),
                Y:       uint32(ty),
                Bytes:   (*bufp)[:n],
                Release: func() { pool.Put(bufp) },
            }
            if err := emit(t); err != nil {
                pool.Put(bufp)
                return err
            }
        }
    }
    return nil
},
Process: func(t pipeline.Tile) (pipeline.Tile, error) {
    rgb := make([]byte, tileBytes)
    rgbOut, err := dec.DecodeTile(t.Bytes, rgb, 1, 1)
    if t.Release != nil {
        t.Release()
        t.Release = nil
    }
    if err != nil {
        return pipeline.Tile{}, err
    }
    encoded, err := enc.EncodeTile(rgbOut, tileW, tileH, nil)
    if err != nil {
        return pipeline.Tile{}, err
    }
    t.Bytes = encoded
    return t, nil
},
```

Key invariants:

- The pool is per-level. When `transcodeOneLevel` returns, the pool falls
  out of scope and is GC'd. No cross-level buffer reuse — adjacent levels
  often have different `TileMaxSize()` values, so reuse would mis-size.
- `Release` is called exactly once per tile under the success path and
  under producer-side error before `emit`. The producer covers the
  no-emit error paths; the worker covers the post-decode normal path.
- **Cancellation-path leak is accepted.** On context cancellation,
  buffers already in the `in` channel never reach a worker and their
  `Release` is never called. Bound: at most `Workers*2` (in-channel) +
  `Workers*2` (out-channel) buffers. Since the pool is per-level and
  `transcodeOneLevel` returns immediately on cancellation, the pool
  itself falls out of scope and the leaked buffers are GC'd shortly
  after. We do not add drain-on-cancel logic to the pipeline package —
  the bounded leak is preferable to coupling pipeline to lifecycle
  semantics it shouldn't know about.
- `pool.Get` returns a buffer of length `TileMaxSize()`. We slice to `n`
  before emitting, but put back the underlying buffer (via the pointer)
  unsliced — `sync.Pool` of `*[]byte` (not `[]byte`) avoids a known Go
  perf cliff where the runtime can't track slice headers in pools.
- The encoded output (`enc.EncodeTile`) is a fresh codec-owned allocation
  per tile. We do not pool it — encoded sizes vary widely between codecs
  and across tiles, and pooling that side is YAGNI for now.

### Downsample buffer reuse

`cmd/wsi-tools/downsample.go` runs sequentially. Hoist a single buffer:

```go
buf := make([]byte, srcL0.TileMaxSize())
for ty := 0; ty < srcGrid.H; ty++ {
    for tx := 0; tx < srcGrid.W; tx++ {
        // ... ctx check ...
        n, err := srcL0.TileInto(tx, ty, buf)
        if err != nil {
            return fmt.Errorf("read source tile (%d,%d): %w", tx, ty, err)
        }
        compressed := buf[:n]
        // ... existing decode + accumulate ...
    }
}
```

No pool, no lifecycle plumbing — single goroutine, lifetime is the loop.

### Round-trip integration test

In `tests/integration/transcode_test.go`, the 4 novel-codec cases currently
shell out to `tiffinfo` for structural validation. Replace with an
opentile-go round-trip:

```go
// After transcode produces outPath:
tlr, err := opentile.OpenFile(outPath)
require.NoError(t, err)
defer tlr.Close()

require.Equal(t, opentile.FormatGenericTIFF, tlr.Format())
levels := tlr.Levels()
require.NotEmpty(t, levels)
require.Equal(t, expectedCompression, levels[0].Compression())  // per-codec
require.Equal(t, srcTileSize, levels[0].TileSize())             // preserved

md := tlr.Metadata()
require.Equal(t, srcMagnification, md.Magnification)
require.NotZero(t, md.AcquisitionDateTime)
```

This validates four things at once: (1) the file is structurally valid TIFF
opentile-go can parse, (2) the format-classifier picks `FormatGenericTIFF`,
(3) the new compression tag mappings work, (4) the wsi-tools
`ImageDescription` parser finds and parses our metadata.

The JPEG-codec test path stays as-is — it already round-trips through
opentile-go's SVS reader since v0.2 used SVS-shaped output for `--codec jpeg`.

### Versioning

Target **v0.3.0**. Rationale: this is additive feature work (new
capabilities claimed) plus an internal-only API change to
`internal/source.Level` (no external callers). Per semver-with-internal-
spirit, MINOR is appropriate. The CHANGELOG entry replaces "opentile-go
novel-codec decoders" in v0.2.0's deferred list with a forward reference
to v0.3.0.

## Testing

- All existing unit + integration tests must continue to pass.
- The 4 novel-codec round-trip cases in `tests/integration/transcode_test.go`
  switch from tiffinfo to opentile-go and gain stronger assertions
  (compression enum + metadata parse), as detailed above.
- A new unit test for the producer's pool lifecycle: verify that
  `Release` is called exactly once per tile (counter-based test against a
  trivial pool stub) under normal flow and under producer-error.
- `go test -race -count=1 ./...` should be clean — the pool-of-`*[]byte`
  pattern is race-free as long as the buffer is owned by exactly one
  goroutine at a time, which the producer→worker hand-off guarantees.

## Risks

- **Pool ownership errors.** A misrouted `Release` (called twice, or
  missed) is silent — no immediate failure, just a buffer leak or use-
  after-put-back. Mitigation: the unit test above; nil out `Release` on
  the worker after invoking it; keep the lifecycle local to a small
  number of callsites.
- **Round-trip test brittleness.** If opentile-go's wsi-tools
  ImageDescription parser tightens up in a future minor release, our
  metadata assertions could break. Mitigation: the parser is documented
  as "lenient on missing/malformed"; assert presence (not exact form) of
  the metadata fields.
- **TileMaxSize variability.** `TileMaxSize()` is an upper bound. For
  pathological compressed-tile sizes (rare), it could be substantially
  larger than the typical tile. We accept the over-allocation; the pool
  reuses across many tiles, amortising.

## Migration sequencing

This work happens on `feat/v0.3-opentile-v014`. The release follows the
same shape as v0.2: spec → plan → implementation → CHANGELOG → tag/release.
