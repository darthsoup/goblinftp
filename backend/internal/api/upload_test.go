package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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

func TestUploadSimple(t *testing.T) {
	var uploadedPath string
	var uploadedContent string

	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/", nil },
		ChmodFn:      func(string, uint32) error { return nil },
		UploadFn: func(path string, r io.Reader) error {
			uploadedPath = path
			data, _ := io.ReadAll(r)
			uploadedContent = string(data)
			return nil
		},
	}
	dialFn := func(p, a, u, pw string, passive bool) (transfer.Client, error) { return mock, nil }
	app, _, _ := newTestApp(t, defaultTestConfig(), api.WithDial(dialFn))
	sess := connectAndGetSession(t, app)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("path", "/uploads/test.txt")
	part, _ := writer.CreateFormFile("file", "test.txt")
	_, _ = io.WriteString(part, "file contents here")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addSession(req, sess)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "/uploads/test.txt", uploadedPath)
	assert.Equal(t, "file contents here", uploadedContent)
}

func TestUploadChunked(t *testing.T) {
	var assembled string
	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/", nil },
		ChmodFn:      func(string, uint32) error { return nil },
		UploadFn: func(path string, r io.Reader) error {
			data, _ := io.ReadAll(r)
			assembled = string(data)
			return nil
		},
	}
	dialFn := func(p, a, u, pw string, passive bool) (transfer.Client, error) { return mock, nil }
	app, _, _ := newTestApp(t, defaultTestConfig(), api.WithDial(dialFn))
	sess := connectAndGetSession(t, app)

	// Reserve
	reserveBody := `{"path":"/big.bin","totalChunks":2,"totalSize":10,"chunkSize":5}`
	reserveReq := httptest.NewRequest(http.MethodPost, "/api/files/upload/reserve", strings.NewReader(reserveBody))
	reserveReq.Header.Set("Content-Type", "application/json")
	addSession(reserveReq, sess)
	reserveRec := httptest.NewRecorder()
	app.ServeHTTP(reserveRec, reserveReq)
	require.Equal(t, http.StatusOK, reserveRec.Code, "reserve: %s", reserveRec.Body.String())

	var reserveResp struct {
		Data struct{ UploadID string `json:"uploadId"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(reserveRec.Body.Bytes(), &reserveResp))
	uploadID := reserveResp.Data.UploadID
	require.NotEmpty(t, uploadID)

	// Send chunks
	sendChunk := func(n int, data string) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("uploadId", uploadID)
		_ = writer.WriteField("chunkIndex", fmt.Sprintf("%d", n))
		part, _ := writer.CreateFormFile("chunk", "chunk")
		_, _ = io.WriteString(part, data)
		writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/files/upload/chunk", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		addSession(req, sess)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "chunk %d: %s", n, rec.Body.String())
	}
	sendChunk(0, "hello")
	sendChunk(1, "world")

	// Commit
	commitBody := fmt.Sprintf(`{"uploadId":%q}`, uploadID)
	commitReq := httptest.NewRequest(http.MethodPost, "/api/files/upload/commit", strings.NewReader(commitBody))
	commitReq.Header.Set("Content-Type", "application/json")
	addSession(commitReq, sess)
	commitRec := httptest.NewRecorder()
	app.ServeHTTP(commitRec, commitReq)
	require.Equal(t, http.StatusOK, commitRec.Code, "commit: %s", commitRec.Body.String())
	assert.Equal(t, "helloworld", assembled)
}
