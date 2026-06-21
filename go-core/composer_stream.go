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
// If done is non-nil, the goroutine exits when done is closed.
func StreamMap[In, Out any](input <-chan In, fn func(In) Out, done <-chan struct{}) <-chan Out {
	output := make(chan Out, defaultBufferSize)
	go func() {
		defer close(output)
		for {
			select {
			case <-done:
				return
			case v, ok := <-input:
				if !ok {
					return
				}
				output <- fn(v)
			}
		}
	}()
	return output
}

// StreamFilter filters elements that don't satisfy the predicate.
// If done is non-nil, the goroutine exits when done is closed.
func StreamFilter[T any](input <-chan T, predicate func(T) bool, done <-chan struct{}) <-chan T {
	output := make(chan T, defaultBufferSize)
	go func() {
		defer close(output)
		for {
			select {
			case <-done:
				return
			case v, ok := <-input:
				if !ok {
					return
				}
				if predicate(v) {
					output <- v
				}
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
// If done is non-nil, the goroutine exits when done is closed.
func Window[T any](input <-chan T, size int, done <-chan struct{}) <-chan []T {
	output := make(chan []T, defaultBufferSize)
	go func() {
		defer close(output)
		window := make([]T, 0, size)
		for {
			select {
			case <-done:
				return
			case v, ok := <-input:
				if !ok {
					if len(window) > 0 {
						output <- window
					}
					return
				}
				window = append(window, v)
				if len(window) == size {
					output <- window
					window = make([]T, 0, size)
				}
			}
		}
	}()
	return output
}

// WindowByTime collects elements within a time window.
// If done is non-nil, the goroutine exits when done is closed.
func WindowByTime[T any](input <-chan T, duration time.Duration, done <-chan struct{}) <-chan []T {
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
			case <-done:
				flush()
				return
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
// If done is non-nil, the goroutine exits when done is closed.
func Merge[T any](done <-chan struct{}, inputs ...<-chan T) <-chan T {
	output := make(chan T, defaultBufferSize)
	var wg sync.WaitGroup
	wg.Add(len(inputs))

	for _, input := range inputs {
		go func(ch <-chan T) {
			defer wg.Done()
			for v := range ch {
				select {
				case <-done:
					return
				case output <- v:
				}
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
// If done is non-nil, the goroutine exits when done is closed.
func Split[T any](input <-chan T, fn func(T) int, n int, done <-chan struct{}) []<-chan T {
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
		for {
			select {
			case <-done:
				return
			case v, ok := <-input:
				if !ok {
					return
				}
				idx := fn(v)
				if idx < 0 {
					idx = 0
				}
				if idx >= n {
					idx = n - 1
				}
				outputs[idx] <- v
			}
		}
	}()

	return result
}

// FromSlice creates a stream from a slice.
// If done is non-nil, the goroutine exits when done is closed.
func FromSlice[T any](items []T, done <-chan struct{}) <-chan T {
	output := make(chan T, defaultBufferSize)
	go func() {
		defer close(output)
		for {
			select {
			case <-done:
				return
			default:
				for _, item := range items {
					select {
					case <-done:
						return
					case output <- item:
					}
				}
				return
			}
		}
	}()
	return output
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

// ToSlice collects stream elements into a slice (alias for Collect).
func ToSlice[T any](input <-chan T) []T {
	return Collect(input)
}
