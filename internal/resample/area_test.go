package resample

import "testing"

func TestAreaAverage_SimpleCorrectness(t *testing.T) {
	// 2x2 source → 1x1 output. Pixels:
	// (10,20,30) (40,50,60)
	// (70,80,90) (100,110,120)
	src := []byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120}
	dst, err := Area2x2(src, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	// avg R = (10+40+70+100)/4 = 55
	// avg G = (20+50+80+110)/4 = 65
	// avg B = (30+60+90+120)/4 = 75
	if dst[0] != 55 || dst[1] != 65 || dst[2] != 75 {
		t.Errorf("got %v, want [55 65 75]", dst[:3])
	}
}

func TestAreaAverage_4x4(t *testing.T) {
	// 4x4 source, all pixels = (100, 100, 100). Output 2x2, all (100,100,100).
	src := make([]byte, 4*4*3)
	for i := range src {
		src[i] = 100
	}
	dst, err := Area2x2(src, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(dst) != 2*2*3 {
		t.Errorf("dst len: got %d, want %d", len(dst), 2*2*3)
	}
	for _, b := range dst {
		if b != 100 {
			t.Errorf("got %d, want 100 in dst", b)
			break
		}
	}
}

func TestAreaAverage_OddDims(t *testing.T) {
	// 3x3 should error (must be even).
	src := make([]byte, 3*3*3)
	if _, err := Area2x2(src, 3, 3); err == nil {
		t.Errorf("expected error for odd dimensions")
	}
}

func TestLanczosStub(t *testing.T) {
	if _, err := Lanczos(nil, 0, 0, 0, 0); err == nil {
		t.Errorf("expected ErrNotImplemented")
	}
}
