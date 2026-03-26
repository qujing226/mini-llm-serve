package errors

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
)

type Code string

const (
	CodeInvalidArgument     Code = "invalid_argument"
	CodeQueueFull           Code = "queue_full"
	CodeRequestTimeout      Code = "request_timeout"
	CodeRequestCanceled     Code = "request_canceled"
	CodeExecutorUnavailable Code = "executor_unavailable"
	CodeInternal            Code = "internal"
)

type AppError struct {
	Code      Code
	Message   string
	Op        string
	Cause     error
	Retryable bool
}

func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case e.Op != "" && e.Message != "":
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	case e.Message != "":
		return e.Message
	case e.Op != "":
		return e.Op
	case e.Cause != nil:
		return e.Cause.Error()
	default:
		return string(e.Code)
	}
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(code Code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

func Wrap(code Code, op, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Op:      op,
		Message: message,
		Cause:   cause,
	}
}

func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	switch {
	case errors.Is(err, context.Canceled):
		return CodeRequestCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return CodeRequestTimeout
	default:
		return CodeInternal
	}
}

func ToConnectCode(err error) connect.Code {
	switch CodeOf(err) {
	case CodeInvalidArgument:
		return connect.CodeInvalidArgument
	case CodeQueueFull:
		return connect.CodeResourceExhausted
	case CodeRequestTimeout:
		return connect.CodeDeadlineExceeded
	case CodeRequestCanceled:
		return connect.CodeCanceled
	case CodeExecutorUnavailable:
		return connect.CodeUnavailable
	case CodeInternal:
		return connect.CodeInternal
	default:
		return connect.CodeUnknown
	}
}

func ToConnectError(err error) error {
	if err == nil {
		return nil
	}
	return connect.NewError(ToConnectCode(err), err)
}
