package source

import "testing"

func TestCompressionString_NewValues(t *testing.T) {
	cases := []struct {
		c    Compression
		want string
	}{
		{CompressionWebP, "webp"},
		{CompressionJPEGXL, "jpegxl"},
		{CompressionHTJ2K, "htj2k"},
	}
	for _, tc := range cases {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("Compression(%d).String() = %q, want %q", tc.c, got, tc.want)
		}
	}
}
