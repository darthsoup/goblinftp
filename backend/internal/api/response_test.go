// backend/internal/api/response_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darthsoup/goblinftp/internal/api"
	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newContext(method, path string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestOKResponse(t *testing.T) {
	c, rec := newContext(http.MethodGet, "/")
	err := api.OK(c, map[string]string{"foo": "bar"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Nil(t, resp.Errors)
}

func TestOKResponseNilData(t *testing.T) {
	c, rec := newContext(http.MethodGet, "/")
	err := api.OK(c, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestFailResponse(t *testing.T) {
	c, rec := newContext(http.MethodPost, "/")
	gftperr := gftperrors.New(gftperrors.ErrAuthFailed, "bad credentials")
	err := api.Fail(c, gftperr)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	require.Len(t, resp.Errors, 1)
	assert.Equal(t, string(gftperrors.ErrAuthFailed), resp.Errors[0].Code)
	assert.Equal(t, "bad credentials", resp.Errors[0].Message)
}

func TestNotImplementedResponse(t *testing.T) {
	c, rec := newContext(http.MethodGet, "/api/files")
	err := api.NotImplemented(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}
