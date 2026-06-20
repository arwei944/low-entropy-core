// Package core — 错误处理增强 (v0.9.0)
//
// 提供企业级错误处理能力：
//   - 堆栈跟踪: 捕获调用栈信息
//   - HTTP 状态码映射: ErrorCategory → HTTP Status Code
//   - gRPC 状态码映射: ErrorCategory → gRPC Status Code
//   - 错误元数据: 附加请求上下文信息
//   - 哨兵错误: 预定义标准错误值

package core

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

// ============================================================================
// 堆栈跟踪
// ============================================================================

// StackTrace 表示调用栈信息。
type StackTrace struct {
	Frames []StackFrame `json:"frames"`
	Raw    string       `json:"raw"`
}

// StackFrame 单个栈帧。
type StackFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// CaptureStackTrace 捕获当前调用栈。
// skip: 跳过的帧数（0 = 当前函数, 1 = 调用者）
// depth: 捕获的最大深度
func CaptureStackTrace(skip, depth int) *StackTrace {
	if depth <= 0 {
		depth = 32
	}

	pcs := make([]uintptr, depth)
	n := runtime.Callers(skip+2, pcs)
	pcs = pcs[:n]

	frames := runtime.CallersFrames(pcs)
	st := &StackTrace{
		Frames: make([]StackFrame, 0, n),
	}

	var sb strings.Builder
	for {
		frame, more := frames.Next()
		st.Frames = append(st.Frames, StackFrame{
			File:     frame.File,
			Line:     frame.Line,
			Function: frame.Function,
		})
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}

	st.Raw = sb.String()
	return st
}

// ============================================================================
// 增强错误类型
// ============================================================================

// RichError 增强的错误类型，包含元数据、堆栈和分类。
type RichError struct {
	Message  string       `json:"message"`
	Category ErrorCategory `json:"category"`
	Cause    error        `json:"cause,omitempty"`
	Stack    *StackTrace  `json:"-"` // 不序列化堆栈
	Metadata map[string]any `json:"metadata,omitempty"`
	Code     string       `json:"code,omitempty"` // 错误码
}

// Error 实现 error 接口。
func (e *RichError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap 接口。
func (e *RichError) Unwrap() error {
	return e.Cause
}

// WithMetadata 附加元数据。
func (e *RichError) WithMetadata(key string, value any) *RichError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// NewRichError 创建增强错误。
func NewRichError(code string, message string, category ErrorCategory) *RichError {
	return &RichError{
		Message:  message,
		Category: category,
		Code:     code,
		Stack:    CaptureStackTrace(1, 32),
	}
}

// WrapRichError 包装已有错误。
func WrapRichError(code string, message string, category ErrorCategory, cause error) *RichError {
	return &RichError{
		Message:  message,
		Category: category,
		Cause:    cause,
		Code:     code,
		Stack:    CaptureStackTrace(1, 32),
	}
}

// ============================================================================
// HTTP / gRPC 状态码映射
// ============================================================================

// HTTPStatusCode 将 ErrorCategory 映射为 HTTP 状态码。
func HTTPStatusCode(category ErrorCategory) int {
	switch category {
	case CatDebug:
		return http.StatusOK
	case CatInfo, CatSuccess:
		return http.StatusOK
	case CatWarning:
		return http.StatusOK
	case CatError, CategoryRecoverable:
		return http.StatusInternalServerError
	case CatFatal, CategoryUnrecoverable:
		return http.StatusInternalServerError
	case CategoryHumanRequired:
		return http.StatusPreconditionFailed
	default:
		return http.StatusInternalServerError
	}
}

// HTTPStatusCodeFromError 从 error 提取 HTTP 状态码。
func HTTPStatusCodeFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if re, ok := err.(*RichError); ok {
		return HTTPStatusCode(re.Category)
	}
	return http.StatusInternalServerError
}

// gRPCStatusCode 将 ErrorCategory 映射为 gRPC 状态码。
// 返回 gRPC 状态码整数值。
func GRPCStatusCode(category ErrorCategory) int {
	switch category {
	case CatDebug, CatInfo, CatSuccess:
		return 0 // OK
	case CatWarning:
		return 0 // OK
	case CatError, CategoryRecoverable:
		return 13 // Internal
	case CatFatal, CategoryUnrecoverable:
		return 13 // Internal
	case CategoryHumanRequired:
		return 9 // FailedPrecondition
	default:
		return 2 // Unknown
	}
}

// GRPCStatusCodeFromError 从 error 提取 gRPC 状态码。
func GRPCStatusCodeFromError(err error) int {
	if err == nil {
		return 0 // OK
	}
	if re, ok := err.(*RichError); ok {
		return GRPCStatusCode(re.Category)
	}
	return 2 // Unknown
}

// ============================================================================
// 哨兵错误
// ============================================================================

// 预定义的标准错误码。
const (
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeAlreadyExists     = "ALREADY_EXISTS"
	ErrCodeInvalidArgument   = "INVALID_ARGUMENT"
	ErrCodeUnauthenticated   = "UNAUTHENTICATED"
	ErrCodePermissionDenied  = "PERMISSION_DENIED"
	ErrCodeResourceExhausted = "RESOURCE_EXHAUSTED"
	ErrCodeFailedPrecondition = "FAILED_PRECONDITION"
	ErrCodeAborted           = "ABORTED"
	ErrCodeOutOfRange        = "OUT_OF_RANGE"
	ErrCodeUnimplemented     = "UNIMPLEMENTED"
	ErrCodeInternal          = "INTERNAL"
	ErrCodeUnavailable       = "UNAVAILABLE"
	ErrCodeDataLoss          = "DATA_LOSS"
	ErrCodeVerConflict   = "VER_CONFLICT"
	ErrCodeTimeout           = "TIMEOUT"
	ErrCodeCircuitOpen       = "CIRCUIT_OPEN"
	ErrCodeRateLimited       = "RATE_LIMITED"
)

// 哨兵错误 — 可直接用 errors.Is 比较。
// 注意: ErrNotFound, ErrInternal, ErrTimeout, ErrCircuitOpen, ErrRateLimited
// 已在 errors.go 中定义为 ErrorCode 类型，此处不再重复声明。
var (
	ErrAlreadyExists     = &RichError{Code: ErrCodeAlreadyExists, Message: "resource already exists", Category: CatError}
	ErrInvalidArgument   = &RichError{Code: ErrCodeInvalidArgument, Message: "invalid argument", Category: CatError}
	ErrUnauthenticated   = &RichError{Code: ErrCodeUnauthenticated, Message: "unauthenticated", Category: CatError}
	ErrPermissionDenied  = &RichError{Code: ErrCodePermissionDenied, Message: "permission denied", Category: CatError}
	ErrResourceExhausted = &RichError{Code: ErrCodeResourceExhausted, Message: "resource exhausted", Category: CatError}
	ErrFailedPrecondition = &RichError{Code: ErrCodeFailedPrecondition, Message: "failed precondition", Category: CategoryHumanRequired}
	ErrAborted           = &RichError{Code: ErrCodeAborted, Message: "operation aborted", Category: CatError}
	ErrOutOfRange        = &RichError{Code: ErrCodeOutOfRange, Message: "out of range", Category: CatError}
	ErrUnimplemented     = &RichError{Code: ErrCodeUnimplemented, Message: "unimplemented", Category: CatError}
	ErrUnavailable       = &RichError{Code: ErrCodeUnavailable, Message: "service unavailable", Category: CatError}
	ErrDataLoss          = &RichError{Code: ErrCodeDataLoss, Message: "data loss", Category: CatFatal}
	ErrVersionConflict   = &RichError{Code: ErrCodeVerConflict, Message: "version conflict", Category: CatError}

	// 韧性相关哨兵错误
	ErrCircuitBreakerOpen = &RichError{Code: ErrCodeCircuitOpen, Message: "circuit breaker open", Category: CatWarning}
	ErrTooManyRequests    = &RichError{Code: ErrCodeRateLimited, Message: "rate limited", Category: CatWarning}
)

// ============================================================================
// 错误响应格式化
// ============================================================================

// ErrorResponse 标准 HTTP 错误响应格式。
type ErrorResponse struct {
	Error   ErrorDetail `json:"error"`
	TraceID string      `json:"trace_id,omitempty"`
}

// ErrorDetail 错误详情。
type ErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// NewErrorResponse 从 error 创建标准错误响应。
func NewErrorResponse(err error) *ErrorResponse {
	resp := &ErrorResponse{
		Error: ErrorDetail{
			Code:    "UNKNOWN",
			Message: err.Error(),
		},
	}

	if re, ok := err.(*RichError); ok {
		resp.Error.Code = re.Code
		resp.Error.Message = re.Message
		if re.Cause != nil {
			resp.Error.Details = map[string]any{
				"cause": re.Cause.Error(),
			}
		}
		if re.Metadata != nil {
			if resp.Error.Details == nil {
				resp.Error.Details = make(map[string]any)
			}
			for k, v := range re.Metadata {
				resp.Error.Details[k] = v
			}
		}
	}

	return resp
}

// NewVersionConflictError 创建版本冲突错误。
func NewVersionConflictErrorEnhanced(expected, actual int64) error {
	return &RichError{
		Code:     ErrCodeVerConflict,
		Message:  fmt.Sprintf("version conflict: expected %d, got %d", expected, actual),
		Category: CatError,
		Stack:    CaptureStackTrace(1, 32),
		Metadata: map[string]any{
			"expected_version": expected,
			"actual_version":   actual,
		},
	}
}
