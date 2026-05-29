package api_test

import (
	"archive/zip"
	"bytes"
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

func TestExtractZipArchive(t *testing.T) {
	var uploadedFiles []string

	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/", nil },
		ChmodFn:      func(string, uint32) error { return nil },
		UploadFn: func(path string, r io.Reader) error {
			uploadedFiles = append(uploadedFiles, path)
			return nil
		},
		MakeDirFn: func(path string) error { return nil },
	}
	dialFn := func(p, a, u, pw string, passive bool) (transfer.Client, error) { return mock, nil }
	app, _, _ := newTestApp(t, defaultTestConfig(), api.WithDial(dialFn))
	sess := connectAndGetSession(t, app)

	// Build a small zip in memory
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, _ := zw.Create("hello.txt")
	_, _ = io.WriteString(w, "hello")
	zw.Close()

	// Upload via multipart
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("destination", "/extracted/")
	part, _ := writer.CreateFormFile("archive", "test.zip")
	_, _ = io.Copy(part, &zipBuf)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files/extract", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addSession(req, sess)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	require.Len(t, uploadedFiles, 1)
	assert.Equal(t, "/extracted/hello.txt", uploadedFiles[0])
}

func TestCreateZipArchive(t *testing.T) {
	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/", nil },
		ChmodFn:      func(string, uint32) error { return nil },
		StatFn: func(path string) (transfer.FileInfo, error) {
			return transfer.FileInfo{Name: "file.txt", IsDir: false, Size: 5}, nil
		},
		DownloadFn: func(path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("hello")), nil
		},
		UploadFn: func(path string, r io.Reader) error {
			// Must consume the reader to unblock the pipe
			_, _ = io.Copy(io.Discard, r)
			return nil
		},
	}
	dialFn := func(p, a, u, pw string, passive bool) (transfer.Client, error) { return mock, nil }
	app, _, _ := newTestApp(t, defaultTestConfig(), api.WithDial(dialFn))
	sess := connectAndGetSession(t, app)

	body := `{"paths":["/file.txt"],"destination":"/archive.zip"}`
	req := httptest.NewRequest(http.MethodPost, "/api/files/compress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addSession(req, sess)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}
