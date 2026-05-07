// Package pipeline runs decode → process → encode tile workflows over a worker
// pool, with cancellation, backpressure, and atomic sink semantics.
package pipeline

import (
	"context"
	"sync"
)

// Tile is the unit of work flowing through the pipeline.
type Tile struct {
	Level int
	X, Y  uint32
	Bytes []byte
}

type SourceFn func(ctx context.Context, emit func(Tile) error) error
type ProcessFn func(Tile) (Tile, error)
type SinkFn func(Tile) error

type Config struct {
	Workers int
	Source  SourceFn
	Process ProcessFn
	Sink    SinkFn
}

// Run drives the pipeline. The Source goroutine emits tiles into a buffered
// channel of size Workers*2; Workers process goroutines transform tiles in
// parallel; a single Sink goroutine receives processed tiles serially. The
// first error from any goroutine cancels the context and Run returns that
// error after draining.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	in := make(chan Tile, cfg.Workers*2)
	out := make(chan Tile, cfg.Workers*2)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(in)
		err := cfg.Source(ctx, func(t Tile) error {
			select {
			case in <- t:
				return nil
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		})
		if err != nil && context.Cause(ctx) == nil {
			cancel(err)
		}
	}()

	var workWG sync.WaitGroup
	workWG.Add(cfg.Workers)
	for i := 0; i < cfg.Workers; i++ {
		go func() {
			defer workWG.Done()
			for t := range in {
				out2, err := cfg.Process(t)
				if err != nil {
					cancel(err)
					return
				}
				select {
				case out <- out2:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		workWG.Wait()
		close(out)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for t := range out {
			if err := cfg.Sink(t); err != nil {
				cancel(err)
				return
			}
		}
	}()

	wg.Wait()
	if err := context.Cause(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}
