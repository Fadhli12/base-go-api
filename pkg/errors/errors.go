// Package errors provides custom error types for the application.
// These errors are used throughout the application to provide consistent
// error handling and HTTP status code mapping.
package errors

import (
	"errors"
	"net/http"
)

// AppError is a custom error type that includes an HTTP status code
type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	Err        error
}

// Error returns the error message
func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError
func NewAppError(code, message string, status int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

// NewAppErrorWithCause creates a new AppError with a wrapped error
func NewAppErrorWithCause(code, message string, status int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
		Err:        err,
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	if err == nil {
		return false
	}
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError extracts an AppError from an error
func GetAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// Sentinel errors for common application errors
var (
	// ErrNotFound is returned when a resource is not found
	ErrNotFound = NewAppError("NOT_FOUND", "Resource not found", http.StatusNotFound)

	// ErrUnauthorized is returned when the user is not authenticated
	ErrUnauthorized = NewAppError("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)

	// ErrForbidden is returned when the user does not have permission
	ErrForbidden = NewAppError("FORBIDDEN", "Permission denied", http.StatusForbidden)

	// ErrBadRequest is returned when the request is invalid
	ErrBadRequest = NewAppError("BAD_REQUEST", "Invalid request", http.StatusBadRequest)

	// ErrConflict is returned when there is a conflict with the current state
	ErrConflict = NewAppError("CONFLICT", "Resource conflict", http.StatusConflict)

	// ErrInternal is returned when an internal server error occurs
	ErrInternal = NewAppError("INTERNAL_ERROR", "Internal server error", http.StatusInternalServerError)

	// ErrValidation is returned when validation fails
	ErrValidation = NewAppError("VALIDATION_ERROR", "Validation failed", http.StatusUnprocessableEntity)

	// ErrTooManyRequests is returned when rate limit is exceeded
	ErrTooManyRequests = NewAppError("RATE_LIMIT_EXCEEDED", "Too many requests", http.StatusTooManyRequests)
)

// WrapNotFound wraps an error as a not found error
func WrapNotFound(err error, resource string) *AppError {
	return NewAppErrorWithCause("NOT_FOUND", resource+" not found", http.StatusNotFound, err)
}

// WrapUnauthorized wraps an error as an unauthorized error
func WrapUnauthorized(err error) *AppError {
	return NewAppErrorWithCause("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized, err)
}

// WrapForbidden wraps an error as a forbidden error
func WrapForbidden(err error) *AppError {
	return NewAppErrorWithCause("FORBIDDEN", "Permission denied", http.StatusForbidden, err)
}

// WrapBadRequest wraps an error as a bad request error
func WrapBadRequest(err error, message string) *AppError {
	if message == "" {
		message = "Invalid request"
	}
	return NewAppErrorWithCause("BAD_REQUEST", message, http.StatusBadRequest, err)
}

// WrapConflict wraps an error as a conflict error
func WrapConflict(err error, message string) *AppError {
	if message == "" {
		message = "Resource conflict"
	}
	return NewAppErrorWithCause("CONFLICT", message, http.StatusConflict, err)
}

// WrapInternal wraps an error as an internal server error
func WrapInternal(err error) *AppError {
	return NewAppErrorWithCause("INTERNAL_ERROR", "Internal server error", http.StatusInternalServerError, err)
}

// WrapValidation wraps an error as a validation error
func WrapValidation(err error, message string) *AppError {
	if message == "" {
		message = "Validation failed"
	}
	return NewAppErrorWithCause("VALIDATION_ERROR", message, http.StatusUnprocessableEntity, err)
}

// IsNotFound checks if an error is ErrNotFound
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflict checks if an error is ErrConflict
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsUnauthorized checks if an error is ErrUnauthorized
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsForbidden checks if an error is ErrForbidden
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}
