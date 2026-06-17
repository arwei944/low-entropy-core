package core

import (
	"sync"
	"time"
)

const defaultBufferSize = 100

// Stream represents a stream of values that can be processed.
type Stream[T any] struct {
	input  <-chan T
	output chan T
	errs   chan error
}

// StreamConfig configures stream processing.
type StreamConfig struct {
	BufferSize int // default 100
}

// StreamMap applies a function to each element in the stream.
// The output channel is buffered (non-blocking) and closed when input is drained.
func StreamMap[In, Out any](input <-chan In, fn func(In) Out) <-chan Out {
	output := make(chan Out, defaultBufferSize)
	go func() {
		defer close(output)
		for v := range input {
			output <- fn(v)
		}
	}()
	return output
}

// StreamFilter filters elements that don't satisfy the predicate.
// The output channel is buffered (non-blocking) and closed when input is drained.
func StreamFilter[T any](input <-chan T, predicate func(T) bool) <-chan T {
	output := make(chan T, defaultBufferSize)
	go func() {
		defer close(output)
		for v := range input {
			if predicate(v) {
				output <- v
			}
		}
	}()
	return output
}

// StreamReduce aggregates all elements into a single value.
// Returns the initial value if the input channel is empty.
func StreamReduce[T, R any](input <-chan T, initial R, fn func(R, T) R) R {
	result := initial
	for v := range input {
		result = fn(result, v)
	}
	return result
}

// Window collects elements into windows of a given size.
// A partial window is emitted at the end if the total count is not divisible by size.
func Window[T any](input <-chan T, size int) <-chan []T {
	output := make(chan []T, defaultBufferSize)
	go func() {
		defer close(output)
		window := make([]T, 0, size)
		for v := range input {
			window = append(window, v)
			if len(window) == size {
				output <- window
				window = make([]T, 0, size)
			}
		}
		if len(window) > 0 {
			output <- window
		}
	}()
	return output
}

// WindowByTime collects elements within a time window.
// When the input channel closes, any remaining buffered elements are emitted.
func WindowByTime[T any](input <-chan T, duration time.Duration) <-chan []T {
	output := make(chan []T, defaultBufferSize)
	go func() {
		defer close(output)
		window := make([]T, 0)
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		flush := func() {
			if len(window) > 0 {
				output <- window
				window = make([]T, 0)
			}
		}

		for {
			select {
			case v, ok := <-input:
				if !ok {
					flush()
					return
				}
				window = append(window, v)
			case <-ticker.C:
				flush()
			}
		}
	}()
	return output
}

// Merge merges multiple input channels into one.
// The output channel is closed when all input channels have been drained.
func Merge[T any](inputs ...<-chan T) <-chan T {
	output := make(chan T, defaultBufferSize)
	var wg sync.WaitGroup
	wg.Add(len(inputs))

	for _, input := range inputs {
		go func(ch <-chan T) {
			defer wg.Done()
			for v := range ch {
				output <- v
			}
		}(input)
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}

// Split splits a single channel into multiple based on a function.
// The function returns an index; values are clamped to [0, n-1].
// All output channels are closed when the input channel is drained.
func Split[T any](input <-chan T, fn func(T) int, n int) []<-chan T {
	outputs := make([]chan T, n)
	result := make([]<-chan T, n)
	for i := 0; i < n; i++ {
		ch := make(chan T, defaultBufferSize)
		outputs[i] = ch
		result[i] = ch
	}

	go func() {
		defer func() {
			for _, ch := range outputs {
				close(ch)
			}
		}()
		for v := range input {
			idx := fn(v)
			if idx < 0 {
				idx = 0
			}
			if idx >= n {
				idx = n - 1
			}
			outputs[idx] <- v
		}
	}()

	return result
}

// Collect collects all elements from a channel into a slice.
// Returns nil if the channel is nil; returns an empty slice if the channel is empty.
func Collect[T any](input <-chan T) []T {
	if input == nil {
		return nil
	}
	var result []T
	for v := range input {
		result = append(result, v)
	}
	return result
}

// FromSlice creates a stream from a slice.
// The returned channel is closed after all elements have been sent.
func FromSlice[T any](items []T) <-chan T {
	output := make(chan T, defaultBufferSize)
	go func() {
		defer close(output)
		for _, item := range items {
			output <- item
		}
	}()
	return output
}

// ToSlice collects stream elements into a slice (alias for Collect).
func ToSlice[T any](input <-chan T) []T {
	return Collect(input)
}