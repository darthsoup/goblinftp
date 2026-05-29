// backend/internal/transfer/client.go
package transfer

import (
	"errors"
	"io"
)

// FileInfo represents a single remote filesystem entry.
type FileInfo struct {
	Name        string
	Size        int64
	IsDir       bool
	ModTime     int64  // Unix timestamp
	Permissions string // e.g. "drwxr-xr-x"
}

// Client is the unified interface that both FTP and SFTP adapters implement.
// All methods that accept a path expect an absolute path on the remote server.
type Client interface {
	// WorkingDir returns the current working directory.
	WorkingDir() (string, error)
	// List returns the contents of the given directory.
	List(path string) ([]FileInfo, error)
	// Stat returns info for a single path. On FTP, this lists the parent dir
	// and finds the entry by name.
	Stat(path string) (FileInfo, error)
	// MakeDir creates a directory (including parents if necessary).
	MakeDir(path string) error
	// Delete removes a file or directory (recursively if dir).
	Delete(path string) error
	// Rename moves src to dst.
	Rename(src, dst string) error
	// Chmod sets permissions on the given path.
	// Returns ErrPermissionsNotSupported if the server does not support it.
	Chmod(path string, mode uint32) error
	// Download opens a reader for the given file. Caller must close it.
	Download(path string) (io.ReadCloser, error)
	// Upload streams from r into the given path, overwriting if it exists.
	Upload(path string, r io.Reader) error
	// Close terminates the underlying connection.
	Close() error
}

// Sentinel errors returned by adapters. Handlers check these with errors.Is.
var (
	ErrAuthFailed              = errors.New("auth failed")
	ErrConnectionFailed        = errors.New("connection failed")
	ErrPermissionsNotSupported = errors.New("permissions not supported")
)
