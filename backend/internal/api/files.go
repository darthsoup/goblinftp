// backend/internal/api/files.go
package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/darthsoup/goblinftp/internal/auth"
	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/darthsoup/goblinftp/internal/transfer"
)

// clientFromContext extracts the transfer.Client from the session stored by requireSession middleware.
func clientFromContext(c echo.Context) (transfer.Client, bool) {
	sess, ok := c.Get("session").(*auth.Session)
	if !ok {
		return nil, false
	}
	client, ok := sess.Data["client"].(transfer.Client)
	return client, ok
}

func (h *Handler) ListFiles(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	path := c.QueryParam("path")
	if path == "" {
		path = "/"
	}
	files, err := client.List(path)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrListFailed, err.Error()))
	}
	return OK(c, map[string]interface{}{"files": files})
}

func (h *Handler) CreateDirectory(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := c.Bind(&req); err != nil || req.Path == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "path is required"))
	}
	if err := client.MakeDir(req.Path); err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	return OK(c, nil)
}

type deleteResult struct {
	Deleted []string       `json:"deleted"`
	Failed  []deleteFailed `json:"failed"`
}

type deleteFailed struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

func (h *Handler) DeleteFiles(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.Bind(&req); err != nil || len(req.Paths) == 0 {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "paths are required"))
	}
	result := deleteResult{}
	for _, p := range req.Paths {
		if err := client.Delete(p); err != nil {
			result.Failed = append(result.Failed, deleteFailed{Path: p, Error: err.Error()})
		} else {
			result.Deleted = append(result.Deleted, p)
		}
	}
	if len(result.Failed) > 0 {
		return c.JSON(http.StatusMultiStatus, Response{
			Success: len(result.Deleted) > 0,
			Data:    result,
		})
	}
	return OK(c, result)
}

func (h *Handler) RenameFile(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := c.Bind(&req); err != nil || req.From == "" || req.To == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "from and to are required"))
	}
	if err := client.Rename(req.From, req.To); err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	return OK(c, nil)
}

func (h *Handler) CopyFile(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := c.Bind(&req); err != nil || req.From == "" || req.To == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "from and to are required"))
	}
	r, err := client.Download(req.From)
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrFileNotFound, err.Error()))
	}
	defer r.Close()
	if err := client.Upload(req.To, r); err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	return OK(c, nil)
}

func (h *Handler) SetPermissions(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	var req struct {
		Path string  `json:"path"`
		Mode *uint32 `json:"mode"`
	}
	if err := c.Bind(&req); err != nil || req.Path == "" || req.Mode == nil {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "path and mode are required"))
	}
	if err := client.Chmod(req.Path, *req.Mode); err != nil {
		if errors.Is(err, transfer.ErrPermissionsNotSupported) {
			return Fail(c, gftperrors.New(gftperrors.ErrPermissionsNotSupported, "chmod not supported by server"))
		}
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	return OK(c, nil)
}
