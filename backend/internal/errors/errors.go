package errors

import "net/http"

// Code is a machine-readable error identifier sent to clients.
type Code string

const (
	ErrConnectionFailed Code = "ERR_CONNECTION_FAILED"
	ErrAuthFailed       Code = "ERR_AUTH_FAILED"
	ErrLoginThrottled   Code = "ERR_LOGIN_THROTTLED"
	ErrFileNotFound     Code = "ERR_FILE_NOT_FOUND"
	ErrFileExists       Code = "ERR_FILE_EXISTS"
	ErrFilePermission   Code = "ERR_FILE_PERMISSION"
	ErrFileNotWritable  Code = "ERR_FILE_NOT_WRITABLE"
	ErrListFailed       Code = "ERR_LIST_FAILED"
	ErrOperationFailed  Code = "ERR_OPERATION_FAILED"
	ErrBadRequest       Code = "ERR_BAD_REQUEST"
	ErrUnauthorized     Code = "ERR_UNAUTHORIZED"
	ErrForbidden        Code = "ERR_FORBIDDEN"
	ErrInternal         Code = "ERR_INTERNAL"
	ErrNotImplemented   Code = "ERR_NOT_IMPLEMENTED"
	ErrInvalidType      Code = "ERR_INVALID_TYPE"
	ErrLoginDisabled    Code = "ERR_LOGIN_DISABLED"
	ErrCSRFInvalid      Code = "ERR_CSRF_INVALID"
	ErrSessionNotFound  Code = "ERR_SESSION_NOT_FOUND"
	ErrQuotaExceeded    Code = "ERR_QUOTA_EXCEEDED"
)

// GFTPError is a typed error with a machine-readable code and human-readable message.
type GFTPError struct {
	code    Code
	message string
}

// New creates a new GFTPError.
func New(code Code, message string) *GFTPError {
	return &GFTPError{code: code, message: message}
}

// Wrap creates a GFTPError from an existing error, using its Error() string as the message.
func Wrap(code Code, err error) *GFTPError {
	if err == nil {
		return &GFTPError{code: code}
	}
	return &GFTPError{code: code, message: err.Error()}
}

// Code returns the error code.
func (e *GFTPError) Code() Code {
	if e == nil {
		return ""
	}
	return e.code
}

// Error implements the error interface.
func (e *GFTPError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

// HTTPStatus maps the error code to an appropriate HTTP status code.
func (e *GFTPError) HTTPStatus() int {
	if e == nil {
		return http.StatusInternalServerError
	}

	switch e.code {
	case ErrBadRequest, ErrInvalidType:
		return http.StatusBadRequest
	case ErrUnauthorized, ErrAuthFailed, ErrSessionNotFound, ErrCSRFInvalid:
		return http.StatusUnauthorized
	case ErrForbidden, ErrLoginThrottled, ErrFilePermission, ErrLoginDisabled:
		return http.StatusForbidden
	case ErrFileNotFound:
		return http.StatusNotFound
	case ErrFileExists:
		return http.StatusConflict
	case ErrNotImplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
