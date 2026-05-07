// Package resample provides image resampling primitives. v0.1 ships 2x2
// area-average; arbitrary-factor lanczos is reserved for v0.2.
package resample

import "fmt"

// Area2x2 produces an RGB888-packed image at half each dimension of src using
// 2x2 area averaging. src must be RGB888-packed and (srcW, srcH) must both be
// even.
func Area2x2(src []byte, srcW, srcH int) ([]byte, error) {
	if srcW%2 != 0 || srcH%2 != 0 {
		return nil, fmt.Errorf("resample: srcW and srcH must be even, got %dx%d", srcW, srcH)
	}
	if len(src) != srcW*srcH*3 {
		return nil, fmt.Errorf("resample: src length %d != %d*%d*3", len(src), srcW, srcH)
	}
	dstW := srcW / 2
	dstH := srcH / 2
	dst := make([]byte, dstW*dstH*3)
	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			sx := dx * 2
			sy := dy * 2
			i00 := (sy*srcW + sx) * 3
			i01 := (sy*srcW + sx + 1) * 3
			i10 := ((sy+1)*srcW + sx) * 3
			i11 := ((sy+1)*srcW + sx + 1) * 3
			di := (dy*dstW + dx) * 3
			for c := 0; c < 3; c++ {
				sum := uint(src[i00+c]) + uint(src[i01+c]) + uint(src[i10+c]) + uint(src[i11+c])
				dst[di+c] = byte((sum + 2) / 4) // round-to-nearest with +2 offset
			}
		}
	}
	return dst, nil
}
