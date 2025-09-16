package graphql

import "iter"

type queue[T any] struct {
	// primary is the queue currently being iterated
	primary []T
	// secondary is used for new items
	secondary []T
	// i is the index in the primary slice to return on next pop
	i int
}

//nolint:unused
func (q *queue[T]) empty() bool {
	return q.i >= len(q.primary) && len(q.secondary) == 0
}

func (q *queue[T]) all() iter.Seq[T] {
	return func(yield func(v T) bool) {
		for {
			v, ok := q.pop()
			if !ok {
				return
			}
			if !yield(v) {
				return
			}
		}
	}
}

func (q *queue[T]) push(v T) {
	q.secondary = append(q.secondary, v)
}

func (q *queue[T]) pop() (T, bool) {
	var empty T
	// If at the end of the current primary slice then
	// swap primary and secondary.
	if q.i >= len(q.primary) {
		// Swap queues
		q.primary = q.primary[:0]
		q.primary, q.secondary = q.secondary, q.primary
		q.i = 0
		// If we're still at the end after swapping then the queue is empty.
		if q.i >= len(q.primary) {
			return empty, false
		}
	}
	v := q.primary[q.i]
	// Allow the value to be garbage collected
	q.primary[q.i] = empty
	q.i++
	return v, true
}
