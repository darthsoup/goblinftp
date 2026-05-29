// backend/internal/transfer/testutil/mock.go
package testutil

import (
	"io"

	"github.com/darthsoup/goblinftp/internal/transfer"
)

// MockClient is a transfer.Client where each method is a swappable function field.
// Any unset field panics when called — intentional, to catch missed setup in tests.
type MockClient struct {
	WorkingDirFn func() (string, error)
	ListFn       func(path string) ([]transfer.FileInfo, error)
	StatFn       func(path string) (transfer.FileInfo, error)
	MakeDirFn    func(path string) error
	DeleteFn     func(path string) error
	RenameFn     func(src, dst string) error
	ChmodFn      func(path string, mode uint32) error
	DownloadFn   func(path string) (io.ReadCloser, error)
	UploadFn     func(path string, r io.Reader) error
	CloseFn      func() error
	Closed       bool
}

func (m *MockClient) WorkingDir() (string, error)                     { return m.WorkingDirFn() }
func (m *MockClient) List(path string) ([]transfer.FileInfo, error)   { return m.ListFn(path) }
func (m *MockClient) Stat(path string) (transfer.FileInfo, error)     { return m.StatFn(path) }
func (m *MockClient) MakeDir(path string) error                       { return m.MakeDirFn(path) }
func (m *MockClient) Delete(path string) error                        { return m.DeleteFn(path) }
func (m *MockClient) Rename(src, dst string) error                    { return m.RenameFn(src, dst) }
func (m *MockClient) Chmod(path string, mode uint32) error            { return m.ChmodFn(path, mode) }
func (m *MockClient) Download(path string) (io.ReadCloser, error)     { return m.DownloadFn(path) }
func (m *MockClient) Upload(path string, r io.Reader) error           { return m.UploadFn(path, r) }
func (m *MockClient) Close() error {
	m.Closed = true
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

// Verify MockClient implements transfer.Client at compile time.
var _ transfer.Client = (*MockClient)(nil)
