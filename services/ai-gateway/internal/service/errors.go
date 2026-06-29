package service

import "errors"

type Code string

const (
	CodeValidation     Code = "validation_error"
	CodeUnauthorized   Code = "unauthorized"
	CodeForbidden      Code = "forbidden"
	CodeNotFound       Code = "not_found"
	CodeConflict       Code = "conflict"
	CodeRateLimited    Code = "rate_limited"
	CodeDependency     Code = "dependency_error"
	CodeInternal       Code = "internal_error"
	CodeNotImplemented Code = "not_implemented"
)

type AppError struct {
	Code    Code
	Message string
	Fields  map[string]string
	Err     error
}

func (e *AppError) Error() string { return e.Message }

func (e *AppError) Unwrap() error { return e.Err }

func NewError(code Code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func ValidationError(fields map[string]string) *AppError {
	return &AppError{Code: CodeValidation, Message: "request validation failed", Fields: fields}
}

func UnauthorizedError() *AppError {
	return &AppError{Code: CodeUnauthorized, Message: "authentication is required"}
}

func NotFoundError(message string, err error) *AppError {
	return &AppError{Code: CodeNotFound, Message: message, Err: err}
}

func ConflictError(message string, err error) *AppError {
	return &AppError{Code: CodeConflict, Message: message, Err: err}
}

func DependencyError(message string, err error) *AppError {
	return &AppError{Code: CodeDependency, Message: message, Err: err}
}

func RateLimitedError(message string, err error) *AppError {
	return &AppError{Code: CodeRateLimited, Message: message, Err: err}
}

func NotImplementedError(message string) *AppError {
	return &AppError{Code: CodeNotImplemented, Message: message}
}

type ProviderError struct {
	Code               Code
	Message            string
	ProviderStatusCode *int
	Err                error
}

func (e *ProviderError) Error() string { return e.Message }

func (e *ProviderError) Unwrap() error { return e.Err }

func NewProviderError(code Code, message string, providerStatusCode *int, err error) *ProviderError {
	return &ProviderError{
		Code:               code,
		Message:            message,
		ProviderStatusCode: providerStatusCode,
		Err:                err,
	}
}

func OpenAIErrorTypeForCode(code Code) string {
	switch code {
	case CodeValidation:
		return "invalid_request_error"
	case CodeUnauthorized:
		return "authentication_error"
	case CodeForbidden:
		return "permission_error"
	case CodeNotFound:
		return "not_found_error"
	case CodeConflict:
		return "conflict_error"
	case CodeRateLimited:
		return "rate_limit_error"
	case CodeNotImplemented:
		return "not_implemented_error"
	case CodeDependency:
		return "upstream_error"
	default:
		return "internal_error"
	}
}

func Classify(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
