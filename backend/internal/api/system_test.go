// backend/internal/api/system_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemVarsPublic(t *testing.T) {
	app, _, _ := newTestApp(t, defaultTestConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/system/vars", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestSystemVarsNoSession(t *testing.T) {
	app, _, _ := newTestApp(t, defaultTestConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/system/vars", nil)
	// No cookie set — should still work (public route)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}
