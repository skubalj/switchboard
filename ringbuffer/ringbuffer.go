package ringbuffer

import (
	"slices"
)

type RingBuffer[T any] interface {
	Append(T) RingBuffer[T]
	// Copy the given data to an in-order slice
	AsSlice() []T
}

func New[T any](cap int) RingBuffer[T] {
	return make(smallArray[T], 0, cap)
}

type smallArray[T any] []T

func (a smallArray[T]) Append(element T) RingBuffer[T] {
	if len(a) == cap(a) {
		a[0] = element
		return largeArray[T]{arr: a, ptr: 1}
	}
	return append(a, element)
}

func (a smallArray[T]) AsSlice() []T {
	return slices.Clone(a)
}

type largeArray[T any] struct {
	arr []T
	ptr int
}

func (a largeArray[T]) Append(element T) RingBuffer[T] {
	a.arr[a.ptr] = element
	a.ptr = (a.ptr + 1) % len(a.arr)
	return a
}

func (a largeArray[T]) AsSlice() []T {
	out := make([]T, 0, len(a.arr))
	out = append(out, a.arr[a.ptr:]...)
	out = append(out, a.arr[:a.ptr]...)
	return out
}
