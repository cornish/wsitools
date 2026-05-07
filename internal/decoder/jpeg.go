package decoder

/*
#cgo pkg-config: libturbojpeg
#include <stdint.h>
#include <stdlib.h>
#include <turbojpeg.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// JPEG decodes JPEG bytes via libjpeg-turbo. Supports the fixed set of
// 1/N scaling factors the library exposes (most commonly 1/1, 1/2, 1/4, 1/8).
type JPEG struct{}

func NewJPEG() *JPEG { return &JPEG{} }

func (*JPEG) DecodeTile(compressed []byte, dst []byte, scaleNum, scaleDen int) ([]byte, error) {
	if len(compressed) == 0 {
		return nil, fmt.Errorf("decoder/jpeg: empty input")
	}
	handle := C.tjInitDecompress()
	if handle == nil {
		return nil, fmt.Errorf("decoder/jpeg: tjInitDecompress failed")
	}
	defer C.tjDestroy(handle)

	var width, height, subsamp, colorspace C.int
	if rc := C.tjDecompressHeader3(handle,
		(*C.uchar)(unsafe.Pointer(&compressed[0])),
		C.ulong(len(compressed)),
		&width, &height, &subsamp, &colorspace); rc != 0 {
		return nil, fmt.Errorf("decoder/jpeg: tjDecompressHeader3: %s", C.GoString(C.tjGetErrorStr2(handle)))
	}

	// Compute target dimensions per libjpeg-turbo's scaling formula:
	// out = ceil(in * num / den).
	if scaleNum <= 0 || scaleDen <= 0 {
		return nil, fmt.Errorf("decoder/jpeg: invalid scale %d/%d", scaleNum, scaleDen)
	}
	outW := (int(width)*scaleNum + scaleDen - 1) / scaleDen
	outH := (int(height)*scaleNum + scaleDen - 1) / scaleDen
	need := outW * outH * 3
	if cap(dst) < need {
		dst = make([]byte, need)
	} else {
		dst = dst[:need]
	}

	if rc := C.tjDecompress2(handle,
		(*C.uchar)(unsafe.Pointer(&compressed[0])),
		C.ulong(len(compressed)),
		(*C.uchar)(unsafe.Pointer(&dst[0])),
		C.int(outW),
		0, // pitch=0 means tightly packed
		C.int(outH),
		C.TJPF_RGB,
		0,
	); rc != 0 {
		return nil, fmt.Errorf("decoder/jpeg: tjDecompress2: %s", C.GoString(C.tjGetErrorStr2(handle)))
	}

	runtime.KeepAlive(compressed)
	runtime.KeepAlive(dst)
	return dst, nil
}
