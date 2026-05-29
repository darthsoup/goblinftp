// backend/internal/api/router.go
package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/darthsoup/goblinftp/internal/auth"
	"github.com/darthsoup/goblinftp/internal/config"
	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
)

const (
	// SessionCookieName is the cookie name used to identify sessions.
	SessionCookieName = "gftp_session"
)

// Register adds the /healthz route, global middleware, and all /api/* routes to e.
func Register(e *echo.Echo, cfg *config.Config, store *auth.Store, thr *auth.Throttle, opts ...HandlerOption) {
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(cspMiddleware())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	h := newHandler(cfg, store, thr, opts)

	// Public routes (no auth required)
	e.GET("/api/system/vars", h.SystemVars)
	e.GET("/api/files/download", h.DownloadFile)

	apiGroup := e.Group("/api")
	apiGroup.Use(csrfMiddleware(store))

	// Auth
	apiGroup.POST("/auth/connect", h.Connect)
	apiGroup.POST("/auth/disconnect", requireSession(store)(h.Disconnect))

	// File operations — Phase 3
	apiGroup.GET("/files", requireSession(store)(h.ListFiles))
	apiGroup.POST("/files/directory", requireSession(store)(h.CreateDirectory))
	apiGroup.DELETE("/files", requireSession(store)(h.DeleteFiles))
	apiGroup.PATCH("/files/rename", requireSession(store)(h.RenameFile))
	apiGroup.PATCH("/files/copy", requireSession(store)(h.CopyFile))
	apiGroup.PATCH("/files/permissions", requireSession(store)(h.SetPermissions))
	apiGroup.POST("/files/download-token", requireSession(store)(h.IssueDownloadToken))
	apiGroup.POST("/files/download-zip", requireSession(store)(NotImplemented))
	apiGroup.POST("/files/upload", requireSession(store)(h.UploadSimple))
	apiGroup.POST("/files/upload/reserve", requireSession(store)(h.UploadReserve))
	apiGroup.POST("/files/upload/chunk", requireSession(store)(h.UploadChunk))
	apiGroup.POST("/files/upload/commit", requireSession(store)(h.UploadCommit))
	apiGroup.POST("/files/extract", requireSession(store)(NotImplemented))
	apiGroup.POST("/files/zip", requireSession(store)(NotImplemented))

	// System — Phase 3
	apiGroup.POST("/system/settings", requireSession(store)(NotImplemented))
}

// cspMiddleware sets Content-Security-Policy on all responses.
func cspMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(
				"Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:",
			)
			return next(c)
		}
	}
}

// csrfMiddleware validates CSRF tokens on all state-changing requests within the /api group.
// Skips /api/auth/connect (unauthenticated) and read-only methods (GET, HEAD, OPTIONS).
func csrfMiddleware(store *auth.Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				return next(c)
			}
			if c.Path() == "/api/auth/connect" {
				return next(c)
			}

			cookie, err := c.Cookie(SessionCookieName)
			if err != nil {
				return Fail(c, gftperrors.New(gftperrors.ErrCSRFInvalid, "missing session cookie"))
			}
			sess, ok := store.Get(cookie.Value)
			if !ok {
				return Fail(c, gftperrors.New(gftperrors.ErrCSRFInvalid, "invalid or expired session"))
			}
			storedToken, _ := sess.Data[auth.CSRFSessionKey].(string)
			headerToken := c.Request().Header.Get(auth.CSRFHeaderName)
			if !auth.ValidateCSRFToken(storedToken, headerToken) {
				return Fail(c, gftperrors.New(gftperrors.ErrCSRFInvalid, "CSRF token mismatch"))
			}
			return next(c)
		}
	}
}

// requireSession returns Echo middleware that enforces a valid session cookie.
// It stores the *auth.Session in the Echo context under key "session".
func requireSession(store *auth.Store) func(echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(SessionCookieName)
			if err != nil {
				return Fail(c, gftperrors.New(gftperrors.ErrUnauthorized, "not authenticated"))
			}
			sess, ok := store.Get(cookie.Value)
			if !ok {
				return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "session expired or not found"))
			}
			store.Touch(sess.ID)
			c.Set("session", sess)
			return next(c)
		}
	}
}
