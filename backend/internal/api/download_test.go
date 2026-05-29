// backend/internal/api/download_test.go
package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darthsoup/goblinftp/internal/api"
	"github.com/darthsoup/goblinftp/internal/transfer"
	"github.com/darthsoup/goblinftp/internal/transfer/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueDownloadToken(t *testing.T) {
	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/", nil },
		ChmodFn:      func(string, uint32) error { return nil },
	}
	dialFn := func(p, a, u, pw string, passive bool) (transfer.Client, error) { return mock, nil }
	app, _, _ := newTestApp(t, defaultTestConfig(), api.WithDial(dialFn))
	sess := connectAndGetSession(t, app)

	body := `{"path":"/file.txt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/files/download-token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addSession(req, sess)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Success bool   `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Data.Token)
}

func TestDownloadFile(t *testing.T) {
	content := "hello file content"
	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/", nil },
		ChmodFn:      func(string, uint32) error { return nil },
		DownloadFn: func(path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(content)), nil
		},
	}
	dialFn := func(p, a, u, pw string, passive bool) (transfer.Client, error) { return mock, nil }
	app, _, _ := newTestApp(t, defaultTestConfig(), api.WithDial(dialFn))
	sess := connectAndGetSession(t, app)

	// Get a download token
	tokenBody := `{"path":"/file.txt"}`
	tokenReq := httptest.NewRequest(http.MethodPost, "/api/files/download-token", strings.NewReader(tokenBody))
	tokenReq.Header.Set("Content-Type", "application/json")
	addSession(tokenReq, sess)
	tokenRec := httptest.NewRecorder()
	app.ServeHTTP(tokenRec, tokenReq)
	require.Equal(t, http.StatusOK, tokenRec.Code)

	var tokenResp struct {
		Data struct{ Token string `json:"token"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(tokenRec.Body.Bytes(), &tokenResp))
	token := tokenResp.Data.Token

	// Use the token to download (no session cookie needed — public route)
	dlReq := httptest.NewRequest(http.MethodGet, "/api/files/download?token="+token, nil)
	dlRec := httptest.NewRecorder()
	app.ServeHTTP(dlRec, dlReq)

	assert.Equal(t, http.StatusOK, dlRec.Code)
	assert.Equal(t, content, dlRec.Body.String())
}
