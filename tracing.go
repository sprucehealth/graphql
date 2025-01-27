package graphql

import (
	"context"
	"iter"
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
	// Unique if true aggregates traces for the same path. Otherwise, only
	// consecutive traces for the same path are aggregated.
	Unique bool
	mu     sync.Mutex
	traces []*TracePathCount
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
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, tr := range t.traces {
		t.traces[i] = nil
		if tr != nil {
			tr.Path = tr.Path[:0]
			tracePathCountPool.Put(tr)
		}
	}
	t.traces = t.traces[:0]
	countingTracerPool.Put(t)
}

// IterTraces returns an iterator of gathered traces. DO NOT retain
// any of the traces at the time Recycle is called on the tracer.
func (t *CountingTracer) IterTraces() iter.Seq2[int, *TracePathCount] {
	return func(yield func(int, *TracePathCount) bool) {
		t.mu.Lock()
		defer t.mu.Unlock()
		for i, tr := range t.traces {
			if !yield(i, tr) {
				return
			}
		}
	}
}

func (t *CountingTracer) Trace(ctx context.Context, path []string, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.traces) != 0 {
		if t.Unique {
			// Go in reverse order because it's most common that repeated traces happen consecutively.
			for i := len(t.traces) - 1; i >= 0; i-- {
				tr := t.traces[i]
				if slices.Equal(path, tr.Path) {
					tr.Count += tr.Count
					tr.MaxDuration = max(tr.MaxDuration, tr.MaxDuration)
					tr.TotalDuration += tr.TotalDuration
					return
				}
			}
		} else if tr := t.traces[len(t.traces)-1]; slices.Equal(path, tr.Path) {
			// Because resolvers execute serially it's guaranteed that for repeated resolves on a field (i.e. a list)
			// the traces will be executed in order. That is, the previous trace either matches the path or the resolves
			// have moved on to another field.
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
	t.traces = append(t.traces, tc)
}
