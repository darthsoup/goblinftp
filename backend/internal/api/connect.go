// backend/internal/api/connect.go
package api

import (
	"net/http"

	"github.com/darthsoup/goblinftp/internal/auth"
	"github.com/labstack/echo/v4"
)

// Connect handles FTP/SFTP connection requests. Full implementation in Task 8.
func (h *Handler) Connect(c echo.Context) error {
	return NotImplemented(c)
}

// Disconnect terminates the current session.
func (h *Handler) Disconnect(c echo.Context) error {
	sess, ok := c.Get("session").(*auth.Session)
	if ok && sess != nil {
		h.store.Delete(sess.ID)
	}
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	return OK(c, nil)
}
