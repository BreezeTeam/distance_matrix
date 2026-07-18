package persist

import (
	"context"

	"distance-matrix/internal/cache"
)

// Archive is the optional cold edge store (MySQL). Nil means feature off.
type Archive interface {
	// Get looks up a directed edge. ok=false means miss (not an error).
	Get(ctx context.Context, opts cache.LookupOpts, origin, destination [2]float32) (cache.Edge, bool, error)
	// Upsert writes or updates one edge (distance==0 skipped by Store).
	Upsert(ctx context.Context, opts cache.LookupOpts, e cache.Edge) error
	Close() error
}

// AsyncWriter wraps Archive with a non-blocking enqueue for write-through.
type AsyncWriter struct {
	inner Archive
	ch    chan upsertJob
	done  chan struct{}
}

type upsertJob struct {
	opts cache.LookupOpts
	edge cache.Edge
}

// NewAsyncWriter starts a background worker. queueSize <=0 → 1024.
func NewAsyncWriter(inner Archive, queueSize int) *AsyncWriter {
	if queueSize <= 0 {
		queueSize = 1024
	}
	w := &AsyncWriter{
		inner: inner,
		ch:    make(chan upsertJob, queueSize),
		done:  make(chan struct{}),
	}
	go w.loop()
	return w
}

func (w *AsyncWriter) loop() {
	defer close(w.done)
	for job := range w.ch {
		_ = w.inner.Upsert(context.Background(), job.opts, job.edge)
	}
}

// EnqueueUpsert never blocks the matrix path; drops when full.
func (w *AsyncWriter) EnqueueUpsert(opts cache.LookupOpts, e cache.Edge) {
	if w == nil || w.inner == nil {
		return
	}
	select {
	case w.ch <- upsertJob{opts: opts, edge: e}:
	default:
		// drop — matrix latency > archive completeness
	}
}

// Get delegates to inner.
func (w *AsyncWriter) Get(ctx context.Context, opts cache.LookupOpts, origin, destination [2]float32) (cache.Edge, bool, error) {
	if w == nil || w.inner == nil {
		return cache.Edge{}, false, nil
	}
	return w.inner.Get(ctx, opts, origin, destination)
}

// Upsert writes synchronously (tests / flush). Prefer EnqueueUpsert in hot path.
func (w *AsyncWriter) Upsert(ctx context.Context, opts cache.LookupOpts, e cache.Edge) error {
	if w == nil || w.inner == nil {
		return nil
	}
	return w.inner.Upsert(ctx, opts, e)
}

// Close stops the worker after draining queued jobs.
func (w *AsyncWriter) Close() error {
	if w == nil {
		return nil
	}
	close(w.ch)
	<-w.done
	if w.inner != nil {
		return w.inner.Close()
	}
	return nil
}
