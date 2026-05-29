// backend/internal/api/sso.go
package api

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/darthsoup/goblinftp/internal/auth"
	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/darthsoup/goblinftp/internal/sso"
	"github.com/darthsoup/goblinftp/internal/transfer"
)

const ssoPendingKey = "sso_pending"

// tokenHash returns the hex-encoded SHA-256 hash of raw (for replay detection).
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// SSOLogin handles GET /?sso=<token>.
// If no sso param: returns 200 placeholder (SPA serving will be added later).
// If sso param present but SSO disabled: returns 403.
// Otherwise: decrypt token, check replay, create session, redirect to /?
func (h *Handler) SSOLogin(c echo.Context) error {
	raw := c.QueryParam("sso")
	if raw == "" {
		return c.String(http.StatusOK, "GoblinFTP")
	}

	if !h.cfg.SSOEnabled {
		return Fail(c, gftperrors.New(gftperrors.ErrUnauthorized, "SSO is not enabled"))
	}

	payload, err := sso.Decrypt(raw, h.cfg.SSOSecret)
	if err != nil {
		if errors.Is(err, sso.ErrTokenExpired) {
			return Fail(c, gftperrors.New(gftperrors.ErrInvalidToken, "SSO token has expired"))
		}
		return Fail(c, gftperrors.New(gftperrors.ErrInvalidToken, "invalid SSO token"))
	}

	hash := tokenHash(raw)
	if h.ssoUsed.IsUsed(hash) {
		return Fail(c, gftperrors.New(gftperrors.ErrInvalidToken, "SSO token already used"))
	}
	h.ssoUsed.Mark(hash, time.Unix(payload.Exp, 0))

	csrfToken, csrfErr := auth.GenerateCSRFToken()
	if csrfErr != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "could not generate CSRF token"))
	}

	sess, sessErr := h.store.New()
	if sessErr != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "could not create session"))
	}
	sess.Data[auth.CSRFSessionKey] = csrfToken
	sess.Data[ssoPendingKey] = ConnectRequest{
		Protocol: payload.Type,
		Host:     payload.Host,
		Port:     payload.Port,
		Username: payload.Username,
		Password: payload.Password,
	}

	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return c.Redirect(http.StatusFound, "/?")
}

// AuthStatus handles GET /api/auth/status.
// Public endpoint: no requireSession middleware. Manually reads session cookie.
// Returns {connected, ssoAutoConnect, csrfToken}.
func (h *Handler) AuthStatus(c echo.Context) error {
	type statusData struct {
		Connected      bool   `json:"connected"`
		SSOAutoConnect bool   `json:"ssoAutoConnect"`
		CSRFToken      string `json:"csrfToken"`
	}

	result := statusData{}

	cookie, err := c.Cookie(SessionCookieName)
	if err == nil {
		if sess, ok := h.store.Get(cookie.Value); ok {
			_, result.Connected = sess.Data["client"].(transfer.Client)
			_, result.SSOAutoConnect = sess.Data[ssoPendingKey]
			result.CSRFToken, _ = sess.Data[auth.CSRFSessionKey].(string)
		}
	}

	return OK(c, result)
}

// SSOConnect handles POST /api/auth/sso-connect.
// Requires valid session (enforced by requireSession middleware).
// Reads the pending SSO ConnectRequest from session, dials, and returns ConnectData.
func (h *Handler) SSOConnect(c echo.Context) error {
	sess := c.Get("session").(*auth.Session)

	pending, ok := sess.Data[ssoPendingKey].(ConnectRequest)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrUnauthorized, "no pending SSO connection"))
	}

	if gftperr := h.checkIPAllowlist(c); gftperr != nil {
		return Fail(c, gftperr)
	}

	addr := fmt.Sprintf("%s:%d", pending.Host, pending.Port)
	client, dialErr := h.dial(pending.Protocol, addr, pending.Username, pending.Password, pending.Passive)
	if dialErr != nil {
		if errors.Is(dialErr, transfer.ErrAuthFailed) {
			return Fail(c, gftperrors.New(gftperrors.ErrAuthFailed, "authentication failed"))
		}
		return Fail(c, gftperrors.New(gftperrors.ErrConnectionFailed, "could not connect to server"))
	}

	initialDir, wdErr := client.WorkingDir()
	if wdErr != nil {
		_ = client.Close()
		return Fail(c, gftperrors.New(gftperrors.ErrConnectionFailed, "could not get working directory"))
	}

	disableChmod := detectChmod(client, pending.Protocol, initialDir)

	sess.Data["client"] = client
	sess.Data["initialDir"] = initialDir
	delete(sess.Data, ssoPendingKey)

	csrfToken, _ := sess.Data[auth.CSRFSessionKey].(string)

	return OK(c, ConnectData{
		Capabilities:     Capabilities{DisableChmod: disableChmod},
		InitialDirectory: initialDir,
		CSRFToken:        csrfToken,
	})
}
