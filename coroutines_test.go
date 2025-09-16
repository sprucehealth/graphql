package graphql

import (
	"context"
	"errors"
	"testing"
)

func TestCoroutines(t *testing.T) {
	ctx := context.Background()

	t.Run("no pause", func(t *testing.T) {
		co := startCoroutine(ctx, func(ctx context.Context) (any, error) {
			return "Hello", nil
		})
		st := co.run()
		if !st.done {
			t.Fatal("Expected done state")
		}
		if st.res != "Hello" {
			t.Fatalf("Expected \"Hello\" got %q", st.res)
		}
		if st.err != nil {
			t.Fatal("Expected nil error")
		}
	})

	t.Run("pause", func(t *testing.T) {
		co := startCoroutine(ctx, func(ctx context.Context) (any, error) {
			if err := PauseCoroutine(ctx); err != nil {
				return nil, err
			}
			return "World", nil
		})
		st := co.run()
		if st.done {
			t.Fatal("Expected not done state")
		}
		if st.res != nil {
			t.Fatalf("Expected nil got %v", st.res)
		}
		if st.err != nil {
			t.Fatal("Expected nil error")
		}
		st = co.run()
		if !st.done {
			t.Fatal("Expected done state")
		}
		if st.res != "World" {
			t.Fatalf("Expected \"World\" got %v", st.res)
		}
		if st.err != nil {
			t.Fatal("Expected nil error")
		}
	})

	t.Run("stop before start", func(t *testing.T) {
		i := 1
		co := startCoroutine(ctx, func(ctx context.Context) (any, error) {
			i = 2
			if err := PauseCoroutine(ctx); err != nil {
				return nil, err
			}
			return "World", nil
		})
		co.stop()
		if i != 1 {
			t.Fatal("Coroutine executed when it should have stopped")
		}
	})

	t.Run("stop", func(t *testing.T) {
		i := 1
		co := startCoroutine(ctx, func(ctx context.Context) (any, error) {
			i = 2
			if err := PauseCoroutine(ctx); err != nil {
				return nil, err
			}
			i = 3
			return "World", nil
		})
		st := co.run()
		if st.done {
			t.Fatal("Expected not done state")
		}
		if st.res != nil {
			t.Fatalf("Expected nil got %v", st.res)
		}
		if st.err != nil {
			t.Fatal("Expected nil error")
		}
		if i != 2 {
			t.Fatal("Coroutine should have pause")
		}
		co.stop()
		if i != 2 {
			t.Fatal("Coroutine should have stopped at pause")
		}
	})
}

func BenchmarkCoroutineInit(b *testing.B) {
	ctx := context.Background()

	fn := func(ctx context.Context) (any, error) {
		return "Hello", nil
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		co := startCoroutine(ctx, fn)
		st := co.run()
		if !st.done {
			b.Fatal("Expected done state")
		}
		if st.res != "Hello" {
			b.Fatalf("Expected \"Hello\" got %q", st.res)
		}
		if st.err != nil {
			b.Fatal("Expected nil error")
		}
	}
}

func TestCoroutineContext(t *testing.T) {
	ctx := context.Background()
	co := startCoroutine(ctx, func(ctx context.Context) (any, error) {
		if !HasCoroutine(ctx) {
			return "", errors.New("expected a coroutine")
		}
		if HasCoroutine(DisconnectCoroutine(ctx)) {
			return "", errors.New("did not expect coroutine")
		}
		return "Hello", nil
	})
	st := co.run()
	if !st.done {
		t.Fatal("Expected coroutine to complete")
	}
	if st.err != nil {
		t.Fatal(st.err)
	}
}

func BenchmarkCoroutinePause(b *testing.B) {
	ctx := context.Background()

	co := startCoroutine(ctx, func(ctx context.Context) (any, error) {
		for range b.N {
			_ = PauseCoroutine(ctx)
		}
		return "World", nil
	})
	b.ResetTimer()
	b.ReportAllocs()
	var st coroutineState
	for range b.N {
		st = co.run()
		if st.done {
			b.Fatal("Expected not done state")
		}
	}
	if st.done {
		b.Fatal("Expected not done state")
	}
	if st.res != nil {
		b.Fatalf("Expected nil got %v", st.res)
	}
	if st.err != nil {
		b.Fatal("Expected nil error")
	}
	st = co.run()
	if !st.done {
		b.Fatal("Expected done state")
	}
	if st.res != "World" {
		b.Fatalf("Expected \"World\" got %v", st.res)
	}
	if st.err != nil {
		b.Fatal("Expected nil error")
	}
}
