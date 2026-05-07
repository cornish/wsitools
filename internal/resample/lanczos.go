package resample

import "errors"

// ErrNotImplemented is returned by Lanczos at v0.1. The real libvips-backed
// implementation lands in v0.2 alongside arbitrary-factor downsampling.
var ErrNotImplemented = errors.New("resample: lanczos not implemented at v0.1 (use Area2x2 or set --resampler area)")

// Lanczos is the v0.2 entry point for non-power-of-2 downsampling. v0.1 returns
// ErrNotImplemented; the CLI rejects non-power-of-2 factors before reaching here.
func Lanczos(src []byte, srcW, srcH, dstW, dstH int) ([]byte, error) {
	return nil, ErrNotImplemented
}
