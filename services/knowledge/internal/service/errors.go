package service

import "errors"

type Code string

const (
	CodeValidation   Code = "validation_error"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeRateLimited  Code = "rate_limited"
	CodeDependency   Code = "dependency_error"
	CodeInternal     Code = "internal_error"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type AppError struct {
	Code    Code
	Message string
	Fields  map[string]string
	Err     error
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewError(code Code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func ValidationError(message string, fields map[string]string) *AppError {
	return &AppError{Code: CodeValidation, Message: message, Fields: fields}
}

func UnauthorizedError() *AppError {
	return &AppError{Code: CodeUnauthorized, Message: "authentication is required"}
}

func ForbiddenError(message string) *AppError {
	return &AppError{Code: CodeForbidden, Message: message}
}

func NotFoundError(message string) *AppError {
	return &AppError{Code: CodeNotFound, Message: message, Err: ErrNotFound}
}

func DependencyError(message string, err error) *AppError {
	return &AppError{Code: CodeDependency, Message: message, Err: err}
}

func ConflictError(message string, err error) *AppError {
	return &AppError{Code: CodeConflict, Message: message, Err: err}
}

func Classify(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
