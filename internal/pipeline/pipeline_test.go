package pipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestPipelineHappyPath(t *testing.T) {
	const N = 100
	var sourceCalls, sinkCalls atomic.Int64
	src := func(ctx context.Context, emit func(Tile) error) error {
		for i := 0; i < N; i++ {
			sourceCalls.Add(1)
			if err := emit(Tile{Level: 0, X: uint32(i), Y: 0, Bytes: []byte{byte(i)}}); err != nil {
				return err
			}
		}
		return nil
	}
	proc := func(t Tile) (Tile, error) {
		t.Bytes = []byte{t.Bytes[0] * 2}
		return t, nil
	}
	var sum atomic.Int64
	sink := func(t Tile) error {
		sinkCalls.Add(1)
		sum.Add(int64(t.Bytes[0]))
		return nil
	}
	if err := Run(context.Background(), Config{Workers: 4, Source: src, Process: proc, Sink: sink}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := sourceCalls.Load(), int64(N); got != want {
		t.Errorf("source calls: got %d, want %d", got, want)
	}
	if got, want := sinkCalls.Load(), int64(N); got != want {
		t.Errorf("sink calls: got %d, want %d", got, want)
	}
	if sum.Load() == 0 {
		t.Errorf("sum is zero")
	}
}

func TestPipelineErrorCancelsRun(t *testing.T) {
	wantErr := errors.New("boom")
	src := func(ctx context.Context, emit func(Tile) error) error {
		for i := 0; i < 100; i++ {
			if err := emit(Tile{X: uint32(i)}); err != nil {
				return err
			}
		}
		return nil
	}
	proc := func(t Tile) (Tile, error) {
		if t.X == 50 {
			return Tile{}, wantErr
		}
		return t, nil
	}
	sink := func(t Tile) error { return nil }
	err := Run(context.Background(), Config{Workers: 4, Source: src, Process: proc, Sink: sink})
	if !errors.Is(err, wantErr) {
		t.Errorf("err: got %v, want %v", err, wantErr)
	}
}

// TestTile_ReleaseInvokedByProcess proves the canonical lifecycle: the
// producer attaches Release; Process invokes it; the pipeline itself
// never invokes Release (it is opaque to the field). After the run,
// the call count equals the number of emitted tiles.
func TestTile_ReleaseInvokedByProcess(t *testing.T) {
	const N = 17
	var released atomic.Int64

	cfg := Config{
		Workers: 4,
		Source: func(ctx context.Context, emit func(Tile) error) error {
			for i := 0; i < N; i++ {
				if err := emit(Tile{
					Level:   0,
					X:       uint32(i),
					Y:       0,
					Bytes:   []byte{byte(i)},
					Release: func() { released.Add(1) },
				}); err != nil {
					return err
				}
			}
			return nil
		},
		Process: func(t Tile) (Tile, error) {
			if t.Release != nil {
				t.Release()
				t.Release = nil
			}
			t.Bytes = []byte{0xFF}
			return t, nil
		},
		Sink: func(t Tile) error { return nil },
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := released.Load(); got != N {
		t.Errorf("Release invoked %d times, want %d", got, N)
	}
}
