package core

import (
	"sync"
	"time"
)

// ============================================================================
// SECTION 9: Stream — 流处理原语
// ============================================================================

const defaultBufferSize = 100

// StreamConfig configures stream processing.
type StreamConfig struct {
	BufferSize int
}

// StreamMap applies a function to each element in the stream.
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
func StreamReduce[T, R any](input <-chan T, initial R, fn func(R, T) R) R {
	result := initial
	for v := range input {
		result = fn(result, v)
	}
	return result
}

// Window collects elements into windows of a given size.
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
