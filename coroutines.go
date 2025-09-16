package graphql

import (
	"context"
	"errors"
	"sync"
)

type ctxCoroutineKey struct{}

type coroutine struct {
	mu       sync.Mutex
	toCoCh   chan bool
	fromCoCh chan coroutineState
	done     bool
}

type coroutineState struct {
	res  any
	err  error
	pan  any
	done bool
}

var (
	// ErrCoroutineStopped is returned from PauseCoroutine when a coroutine is stopped. The caller
	// should propagate the error up as quickly as possible.
	ErrCoroutineStopped = errors.New("graphql: coroutine stopped")
	// ErrNoCoroutine is returned from PauseCoroutine when there's no coroutine in the context.
	ErrNoCoroutine = errors.New("graphql: no coroutine")
)

func startCoroutine(ctx context.Context, fn func(ctx context.Context) (any, error)) *coroutine {
	co := &coroutine{
		toCoCh:   make(chan bool),
		fromCoCh: make(chan coroutineState),
	}
	ctx = context.WithValue(ctx, ctxCoroutineKey{}, co)
	go func() {
		defer func() {
			if r := recover(); r != nil && !co.done {
				co.done = true
				co.fromCoCh <- coroutineState{
					done: true,
					pan:  r,
				}
			}
		}()

		// Wait for the initial run.
		stop := <-co.toCoCh
		if stop {
			co.done = true
			co.fromCoCh <- coroutineState{
				done: true,
				err:  ErrCoroutineStopped,
			}
			return
		}
		v, err := fn(ctx)
		co.mu.Lock()
		defer co.mu.Unlock()
		co.done = true
		co.fromCoCh <- coroutineState{
			done: true,
			res:  v,
			err:  err,
		}
	}()
	return co
}

func (c *coroutine) run() coroutineState {
	if c.done {
		return coroutineState{
			err:  ErrCoroutineStopped,
			done: true,
		}
	}
	// Signal the coroutine go routine to start.
	c.toCoCh <- false
	// Wait for a response.
	st := <-c.fromCoCh
	// Propagate panics
	if st.pan != nil {
		panic(st.pan)
	}
	return st
}

func (c *coroutine) stop() {
	if c.done {
		return
	}
	// Signal the coroutine go routine to stop.
	c.toCoCh <- true
	// Wait for a response.
	st := <-c.fromCoCh
	// Propagate panics
	if st.pan != nil {
		panic(st.pan)
	}
}

// HasCoroutine returns true if the context contains a coroutine.
func HasCoroutine(ctx context.Context) bool {
	_, ok := ctx.Value(ctxCoroutineKey{}).(*coroutine)
	return ok
}

// PauseCoroutine is called inside a coroutine and pauses execution returning
// control the parent. It returns ErrNoCoroutine when there's no coroutine in the
// context and ErrCoroutineStopped if the coroutine is stopped.
func PauseCoroutine(ctx context.Context) error {
	c, ok := ctx.Value(ctxCoroutineKey{}).(*coroutine)
	if !ok {
		return ErrNoCoroutine
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return ErrCoroutineStopped
	}
	c.fromCoCh <- coroutineState{res: nil, done: false}
	stop := <-c.toCoCh
	if stop {
		c.done = true
		return ErrCoroutineStopped
	}
	return nil
}

// DisconnectCoroutine removes any coroutine from the context. This should be used
// when passing the context to a goroutine that runs concurrently with other code
// that may use the coroutine.
func DisconnectCoroutine(ctx context.Context) context.Context {
	_, ok := ctx.Value(ctxCoroutineKey{}).(*coroutine)
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, ctxCoroutineKey{}, nil)
}
