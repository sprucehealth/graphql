package graphql

import (
	"context"
	"slices"
	"sync"
	"time"
)

var (
	countingTracerPool = sync.Pool{New: func() any { return &CountingTracer{} }}
	tracePathCountPool = sync.Pool{New: func() any { return &TracePathCount{} }}
)

type Tracer interface {
	Trace(ctx context.Context, path []string, duration time.Duration)
}

type CountingTracer struct {
	Traces []*TracePathCount
}

type TracePathCount struct {
	Path          []string
	Count         int
	TotalDuration time.Duration
	MaxDuration   time.Duration
}

func NewCountingTracer() *CountingTracer {
	return countingTracerPool.Get().(*CountingTracer)
}

// Recycle returns the counting tracer and all traces to the pool.
func (t *CountingTracer) Recycle() {
	for i, tr := range t.Traces {
		t.Traces[i] = nil
		tr.Path = tr.Path[:0]
		tracePathCountPool.Put(tr)
	}
	t.Traces = t.Traces[:0]
	countingTracerPool.Put(t)
}

func (t *CountingTracer) Trace(ctx context.Context, path []string, duration time.Duration) {
	if len(t.Traces) != 0 {
		// Because resolvers execute serially it's guaranteed that for repeated resolves on a field (i.e. a list)
		// the traces will be executed in order. That is, the previous trace either matches the path or the resolves
		// have moved on to another field.
		if tr := t.Traces[len(t.Traces)-1]; slices.Equal(path, tr.Path) {
			tr.Count++
			tr.TotalDuration += duration
			tr.MaxDuration = max(tr.MaxDuration, duration)
			return
		}
	}
	tc := tracePathCountPool.Get().(*TracePathCount)
	tc.Path = append(tc.Path, path...)
	tc.Count = 1
	tc.TotalDuration = duration
	tc.MaxDuration = duration
	t.Traces = append(t.Traces, tc)
}
