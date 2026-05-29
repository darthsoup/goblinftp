// backend/internal/transfer/upload.go
package transfer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/google/uuid"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// validateUploadID returns an error if id is not a valid UUID.
func validateUploadID(id string) error {
	if !uuidRe.MatchString(id) {
		return fmt.Errorf("invalid upload ID: %q", id)
	}
	return nil
}

const SessionUploadsKey = "uploads"

// UploadMeta tracks the state of an in-progress chunked upload.
type UploadMeta struct {
	ID             string
	Destination    string
	TotalChunks    int
	ReceivedChunks int
	ChunkSize      int64
}

// UploadStore is a thread-safe in-memory store for upload metadata.
type UploadStore struct {
	mu      sync.Mutex
	uploads map[string]*UploadMeta
}

func NewUploadStore() *UploadStore {
	return &UploadStore{uploads: make(map[string]*UploadMeta)}
}

// Create registers a new upload and returns its ID and a copy of the metadata.
func (s *UploadStore) Create(destination string, totalChunks int) (string, *UploadMeta) {
	id := uuid.NewString()
	meta := &UploadMeta{
		Destination: destination,
		TotalChunks: totalChunks,
	}
	s.mu.Lock()
	s.uploads[id] = meta
	s.mu.Unlock()
	return id, meta
}

// Get returns a pointer to the metadata for uploadID, or (nil, false) if not found.
func (s *UploadStore) Get(id string) (*UploadMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.uploads[id]
	return meta, ok
}

// MarkReceived increments ReceivedChunks for uploadID.
func (s *UploadStore) MarkReceived(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if meta, ok := s.uploads[id]; ok {
		meta.ReceivedChunks++
	}
}

// Delete removes the upload entry from the store.
func (s *UploadStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.uploads, id)
}

// NewUpload creates a temp directory for chunks and returns metadata.
func NewUpload(dataDir, destination string, totalChunks int, chunkSize int64) (*UploadMeta, error) {
	id := uuid.NewString()
	dir := filepath.Join(dataDir, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &UploadMeta{
		ID:          id,
		Destination: destination,
		TotalChunks: totalChunks,
		ChunkSize:   chunkSize,
	}, nil
}

// WriteChunk writes a chunk to the upload directory.
func WriteChunk(dataDir, uploadID string, index int, r io.Reader) error {
	if err := validateUploadID(uploadID); err != nil {
		return err
	}
	name := filepath.Join(dataDir, uploadID, fmt.Sprintf("%04d", index))
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// AssembleReader returns an io.ReadCloser that reads all chunks in order.
func AssembleReader(dataDir, uploadID string, totalChunks int) (io.ReadCloser, error) {
	if err := validateUploadID(uploadID); err != nil {
		return nil, err
	}
	// Verify all chunks exist before opening any
	for i := 0; i < totalChunks; i++ {
		name := filepath.Join(dataDir, uploadID, fmt.Sprintf("%04d", i))
		if _, err := os.Stat(name); err != nil {
			return nil, fmt.Errorf("chunk %d missing: %w", i, err)
		}
	}
	// Then open them
	readers := make([]io.Reader, totalChunks)
	closers := make([]io.Closer, totalChunks)
	for i := 0; i < totalChunks; i++ {
		name := filepath.Join(dataDir, uploadID, fmt.Sprintf("%04d", i))
		f, err := os.Open(name)
		if err != nil {
			// close already opened files
			for j := 0; j < i; j++ {
				closers[j].Close()
			}
			return nil, err
		}
		readers[i] = f
		closers[i] = f
	}
	multi := io.MultiReader(readers...)
	return &multiReadCloser{Reader: multi, closers: closers}, nil
}

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (m *multiReadCloser) Close() error {
	var last error
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			last = err
		}
	}
	return last
}

// Cleanup removes the upload directory.
func Cleanup(dataDir, uploadID string) error {
	if err := validateUploadID(uploadID); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(dataDir, uploadID))
}
