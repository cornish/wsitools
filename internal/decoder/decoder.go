// Package decoder provides RGB-out decoders for the source codecs that v0.1's
// downsample tool needs (JPEG, JPEG 2000). Wider than internal/codec because
// the codec interface assumes encoders; decoders have a smaller surface and
// are used at job-start to feed the resample/encode pipeline.
package decoder

type Decoder interface {
	// DecodeTile decodes compressed bytes into RGB888 packed bytes.
	// scaleNum/scaleDen selects libjpeg-turbo's in-decode fast scale where
	// available (1/1, 1/2, 1/4, 1/8). Other decoders ignore the scaling
	// factor (or return an error if it isn't 1/1).
	// dst is an optional output buffer; if cap(dst) is large enough,
	// decode in-place and return dst[:N]; otherwise return a fresh slice.
	DecodeTile(compressed []byte, dst []byte, scaleNum, scaleDen int) ([]byte, error)
}
