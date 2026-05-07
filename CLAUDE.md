# wsi-tools

Go-based utilities for whole-slide imaging (WSI) files. v0.1 ships a downsample CLI;
v0.2+ adds transcode + more source formats.

## Module path

`github.com/cornish/wsi-tools`

## Conventions

- Reader = `github.com/cornish/opentile-go` (consumed as a Go module dep, not forked).
- Writer = `internal/wsiwriter` (pure Go for TIFF structure; cgo only inside codec wrappers).
- Codecs = `internal/codec/<codec>/` subpackages, one per codec, registered via `init()`.
- Decoders = `internal/decoder/` (smaller surface — only what source slides need).
- Pipeline = `internal/pipeline` (worker-pool decode/process/encode).
- CLI = `cmd/wsi-tools/` using cobra.

## Test discipline

- `make test` runs with `-race -count=1`.
- Integration tests gated by `WSI_TOOLS_TESTDIR` env var (default `./sample_files`).
- `sample_files/` is gitignored; soft-link to opentile-go's pool:

  ```sh
  ln -s "$HOME/GitHub/opentile-go/sample_files" sample_files
  ```

## No guessing

When unsure about TIFF byte layout, Aperio ImageDescription, or any WSI quirk: read
the opentile-go reader source first; it's canonical. The spec rule from opentile-go's
CLAUDE.md applies here too — don't reason from first principles about WSI formats,
read the reference implementation.

## Spec + plans

Design docs live at `docs/superpowers/specs/`; implementation plans at
`docs/superpowers/plans/`.
