package graphql

import (
	"slices"
	"testing"
)

func TestQueue(t *testing.T) {
	cases := map[string][]struct {
		act string
		v   int
		ok  bool
	}{
		"empty pop": {
			{act: "pop", v: 0, ok: false},
		},
		"push pop pop": {
			{act: "push", v: 1},
			{act: "pop", v: 1, ok: true},
			{act: "pop", v: 0, ok: false},
		},
		"push push pop pop pop": {
			{act: "push", v: 1},
			{act: "push", v: 2},
			{act: "pop", v: 1, ok: true},
			{act: "pop", v: 2, ok: true},
			{act: "pop", v: 0, ok: false},
		},
		"push push pop push pop push pop pop": {
			{act: "push", v: 1},
			{act: "push", v: 2},
			{act: "pop", v: 1, ok: true},
			{act: "push", v: 3, ok: true},
			{act: "pop", v: 2, ok: true},
			{act: "push", v: 4, ok: true},
			{act: "pop", v: 3, ok: true},
			{act: "pop", v: 4, ok: true},
			{act: "pop", v: 0, ok: false},
		},
	}
	for name, actions := range cases {
		t.Run(name, func(t *testing.T) {
			var q queue[int]
			for _, a := range actions {
				switch a.act {
				case "push":
					q.push(a.v)
				case "pop":
					v, ok := q.pop()
					if ok != a.ok {
						t.Fatalf("Expected ok %t for Pop", a.ok)
					}
					if v != a.v {
						t.Fatalf("Expected value %v got Pop, got %v", a.v, v)
					}
				default:
					t.Fatalf("Unknown action %q", a.act)
				}
			}
		})
	}
}

func TestQueueAll(t *testing.T) {
	var q queue[int]
	var sl []int
	for v := range q.all() {
		sl = append(sl, v)
	}
	if len(sl) != 0 {
		t.Fatalf("Expected empty slice, got %v", sl)
	}
	q.push(1)
	q.push(2)
	for v := range q.all() {
		sl = append(sl, v)
	}
	exp := []int{1, 2}
	if !slices.Equal(sl, exp) {
		t.Fatalf("Expected %v, got %v", exp, sl)
	}
}

func BenchmarkQueuePush(b *testing.B) {
	b.ReportAllocs()
	var q queue[int]
	for b.Loop() {
		q.push(1)
	}
}

func BenchmarkQueuePop(b *testing.B) {
	var q queue[int]
	for i := range b.N {
		q.push(i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = q.pop()
	}
}

func BenchmarkQueuePopEmpty(b *testing.B) {
	var q queue[int]
	b.ReportAllocs()
	for b.Loop() {
		_, _ = q.pop()
	}
}

func BenchmarkQueuePushPop(b *testing.B) {
	b.ReportAllocs()
	var q queue[int]
	for b.Loop() {
		q.push(1)
		_, _ = q.pop()
	}
}

func BenchmarkQueuePushPopLongQueue1000(b *testing.B) {
	b.ReportAllocs()
	var q queue[int]
	for range 1000 {
		q.push(1)
	}
	b.ResetTimer()
	for b.Loop() {
		q.push(1)
		_, _ = q.pop()
	}
}
