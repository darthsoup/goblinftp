// backend/internal/api/download.go
package api

import (
	"io"
	"net/http"
	"path"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/darthsoup/goblinftp/internal/auth"
	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/darthsoup/goblinftp/internal/transfer"
)

// IssueDownloadToken issues a signed short-lived token for downloading a file.
func (h *Handler) IssueDownloadToken(c echo.Context) error {
	sess, ok := c.Get("session").(*auth.Session)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active session"))
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := c.Bind(&req); err != nil || req.Path == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "path is required"))
	}
	expiry := time.Now().Add(15 * time.Minute)
	tok, err := transfer.IssueToken(h.cfg.DownloadTokenSecret, sess.ID, req.Path, expiry)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to issue token"))
	}
	return OK(c, map[string]string{"token": tok})
}

// DownloadFile is a public endpoint that streams a file using a signed token.
func (h *Handler) DownloadFile(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrInvalidToken, "token is required"))
	}
	sessionID, filePath, err := transfer.ValidateToken(h.cfg.DownloadTokenSecret, token)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInvalidToken, err.Error()))
	}
	sess, ok := h.store.Get(sessionID)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "session not found"))
	}
	client, ok := sess.Data["client"].(transfer.Client)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	r, err := client.Download(filePath)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrFileNotFound, err.Error()))
	}
	defer r.Close()

	filename := path.Base(filePath)
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(c.Response(), r)
	return copyErr
}
