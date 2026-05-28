package errors_test

import (
	"fmt"
	"net/http"
	"testing"

	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	err := gftperrors.New(gftperrors.ErrAuthFailed, "authentication failed")
	assert.Equal(t, gftperrors.ErrAuthFailed, err.Code())
	assert.Equal(t, "authentication failed", err.Error())
}

func TestWrapError(t *testing.T) {
	underlying := fmt.Errorf("connection refused")
	err := gftperrors.Wrap(gftperrors.ErrConnectionFailed, underlying)
	assert.Equal(t, gftperrors.ErrConnectionFailed, err.Code())
	assert.Equal(t, "connection refused", err.Error())
}

func TestWrapNilError(t *testing.T) {
	err := gftperrors.Wrap(gftperrors.ErrInternal, nil)
	assert.Equal(t, gftperrors.ErrInternal, err.Code())
	assert.Equal(t, "", err.Error())
}

func TestGFTPErrorImplementsErrorInterface(t *testing.T) {
	var err error = gftperrors.New(gftperrors.ErrInternal, "oops")
	assert.Equal(t, "oops", err.Error())
}

func TestNilReceiver(t *testing.T) {
	var err *gftperrors.GFTPError
	assert.Equal(t, gftperrors.Code(""), err.Code())
	assert.Equal(t, "", err.Error())
	assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus())
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		code     gftperrors.Code
		expected int
	}{
		{gftperrors.ErrBadRequest, http.StatusBadRequest},
		{gftperrors.ErrInvalidType, http.StatusBadRequest},
		{gftperrors.ErrUnauthorized, http.StatusUnauthorized},
		{gftperrors.ErrAuthFailed, http.StatusUnauthorized},
		{gftperrors.ErrSessionNotFound, http.StatusUnauthorized},
		{gftperrors.ErrCSRFInvalid, http.StatusUnauthorized},
		{gftperrors.ErrForbidden, http.StatusForbidden},
		{gftperrors.ErrLoginThrottled, http.StatusForbidden},
		{gftperrors.ErrFilePermission, http.StatusForbidden},
		{gftperrors.ErrLoginDisabled, http.StatusForbidden},
		{gftperrors.ErrFileNotFound, http.StatusNotFound},
		{gftperrors.ErrFileExists, http.StatusConflict},
		{gftperrors.ErrNotImplemented, http.StatusNotImplemented},
		{gftperrors.ErrInternal, http.StatusInternalServerError},
		{gftperrors.ErrConnectionFailed, http.StatusInternalServerError},
		{gftperrors.ErrOperationFailed, http.StatusInternalServerError},
		{gftperrors.ErrFileNotWritable, http.StatusInternalServerError},
		{gftperrors.ErrListFailed, http.StatusInternalServerError},
		{gftperrors.ErrQuotaExceeded, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		err := gftperrors.New(tt.code, "msg")
		assert.Equal(t, tt.expected, err.HTTPStatus(), "code=%s", tt.code)
	}
}
