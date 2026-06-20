package core

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ============================================================================
// 重试执行器
// ============================================================================

// RetryWithBackoff 使用退避策略重试操作。
// maxRetries: 最大重试次数（0 = 无限重试直到 ctx 取消）
func RetryWithBackoff(ctx context.Context, strategy BackoffStrategy, maxRetries int, fn func() error) error {
	strategy.Reset()

	for attempt := 0; maxRetries == 0 || attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// 检查是否应该重试
		if IsRetryable(err) {
			if attempt == maxRetries {
				return fmt.Errorf("retry: max retries (%d) exceeded: %w", maxRetries, err)
			}

			delay := strategy.NextDelay(attempt)
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry: cancelled: %w", ctx.Err())
			case <-time.After(delay):
				continue
			}
		}

		return err
	}

	return fmt.Errorf("retry: unexpected loop exit")
}

// IsRetryable 检查错误是否可重试。
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// 熔断器打开、限流等临时错误可重试
	if IsErrorCode(err, ErrCircuitOpen.Code) || err == ErrTooManyRequests || err == ErrUnavailable {
		return true
	}
	if re, ok := err.(*RichError); ok {
		return re.Code == ErrCodeUnavailable ||
			re.Code == ErrCodeTimeout ||
			re.Code == ErrCircuitOpen.Code ||
			re.Code == ErrCodeRateLimited
	}
	return false
}

// RetryableError 包装错误为可重试。
func RetryableError(err error) error {
	return &RichError{
		Code:     ErrCodeUnavailable,
		Message:  err.Error(),
		Category: CatWarning,
		Stack:    CaptureStackTrace(1, 32),
	}
}

// ============================================================================
// HTTP 限流中间件
// ============================================================================

// RateLimitMiddleware 创建 HTTP 限流中间件。
func RateLimitMiddleware(limiter *TokenBucketRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", limiter.rate))
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
