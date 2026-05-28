// backend/internal/api/response.go
package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
)

// APIError is a single error entry in a failure response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Response is the standard API envelope for all endpoints.
type Response struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Errors  []APIError `json:"errors,omitempty"`
}

// OK writes a 200 success response with the given data payload.
func OK(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

// Fail writes an error response. The HTTP status code comes from the first error's HTTPStatus().
func Fail(c echo.Context, errs ...*gftperrors.GFTPError) error {
	apiErrors := make([]APIError, len(errs))
	status := http.StatusInternalServerError
	if len(errs) > 0 {
		status = errs[0].HTTPStatus()
	}
	for i, e := range errs {
		apiErrors[i] = APIError{Code: string(e.Code()), Message: e.Error()}
	}
	return c.JSON(status, Response{Success: false, Errors: apiErrors})
}

// NotImplemented returns a 501 stub response for routes planned in future phases.
func NotImplemented(c echo.Context) error {
	return Fail(c, gftperrors.New(gftperrors.ErrNotImplemented, "not implemented in this phase"))
}
