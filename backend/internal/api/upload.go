// backend/internal/api/upload.go
package api

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/darthsoup/goblinftp/internal/auth"
	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/darthsoup/goblinftp/internal/transfer"
)

func (h *Handler) UploadSimple(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	remotePath := c.FormValue("path")
	if remotePath == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "path is required"))
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "file is required"))
	}
	f, err := fh.Open()
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to open file"))
	}
	defer f.Close()
	if err := client.Upload(remotePath, f); err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	return OK(c, nil)
}

func (h *Handler) UploadReserve(c echo.Context) error {
	sess, ok := c.Get("session").(*auth.Session)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active session"))
	}
	var req struct {
		Path        string `json:"path"`
		TotalChunks int    `json:"totalChunks"`
		TotalSize   int64  `json:"totalSize"`
		ChunkSize   int64  `json:"chunkSize"`
	}
	if err := c.Bind(&req); err != nil || req.Path == "" || req.TotalChunks < 1 {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "path and totalChunks are required"))
	}
	meta, err := transfer.NewUpload(h.dataDir, req.Path, req.TotalChunks, req.ChunkSize)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to reserve upload"))
	}
	uploads := getUploadsMap(sess)
	uploads[meta.ID] = meta
	sess.Data[transfer.SessionUploadsKey] = uploads
	return OK(c, map[string]string{"uploadId": meta.ID})
}

func (h *Handler) UploadChunk(c echo.Context) error {
	sess, ok := c.Get("session").(*auth.Session)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active session"))
	}
	uploadID := c.FormValue("uploadId")
	chunkIndexStr := c.FormValue("chunkIndex")
	if uploadID == "" || chunkIndexStr == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "uploadId and chunkIndex are required"))
	}
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "invalid chunkIndex"))
	}
	uploads := getUploadsMap(sess)
	meta, ok := uploads[uploadID]
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrUploadNotFound, "upload not found"))
	}
	if chunkIndex < 0 || chunkIndex >= meta.TotalChunks {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "chunkIndex out of range"))
	}
	fh, err := c.FormFile("chunk")
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "chunk file is required"))
	}
	f, err := fh.Open()
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to open chunk"))
	}
	defer f.Close()
	if err := transfer.WriteChunk(h.dataDir, uploadID, chunkIndex, f); err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to write chunk"))
	}
	return OK(c, nil)
}

func (h *Handler) UploadCommit(c echo.Context) error {
	sess, ok := c.Get("session").(*auth.Session)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active session"))
	}
	client, clientOk := clientFromContext(c)
	if !clientOk {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	var req struct {
		UploadID string `json:"uploadId"`
	}
	if err := c.Bind(&req); err != nil || req.UploadID == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "uploadId is required"))
	}
	uploads := getUploadsMap(sess)
	meta, ok := uploads[req.UploadID]
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrUploadNotFound, "upload not found"))
	}
	r, err := transfer.AssembleReader(h.dataDir, meta.ID, meta.TotalChunks)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to assemble chunks"))
	}
	defer r.Close()
	if err := client.Upload(meta.Destination, r); err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	_ = transfer.Cleanup(h.dataDir, meta.ID)
	delete(uploads, req.UploadID)
	sess.Data[transfer.SessionUploadsKey] = uploads
	return OK(c, nil)
}

// NOTE: Session.Data is not protected by a mutex. Concurrent requests
// to the same session's upload endpoints may race. This is acceptable
// for the typical single-user FTP client use case.
func getUploadsMap(sess *auth.Session) map[string]*transfer.UploadMeta {
	if m, ok := sess.Data[transfer.SessionUploadsKey]; ok {
		if uploads, ok := m.(map[string]*transfer.UploadMeta); ok {
			return uploads
		}
	}
	return make(map[string]*transfer.UploadMeta)
}
