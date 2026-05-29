// backend/internal/api/sso_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/darthsoup/goblinftp/internal/api"
	"github.com/darthsoup/goblinftp/internal/auth"
	"github.com/darthsoup/goblinftp/internal/config"
	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/darthsoup/goblinftp/internal/sso"
	"github.com/darthsoup/goblinftp/internal/transfer"
	"github.com/darthsoup/goblinftp/internal/transfer/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ssoEnabledConfig() *config.Config {
	cfg := defaultTestConfig()
	cfg.SSOEnabled = true
	cfg.SSOSecret = []byte("test-sso-secret-32bytes-xxxxxxxxxxx")
	return cfg
}

// validSSO creates an encrypted SSO token with future expiry.
func validSSO(t *testing.T, secret []byte) string {
	t.Helper()
	payload := &sso.Payload{
		Type:     "ftp",
		Host:     "ftp.example.com",
		Port:     21,
		Username: "user",
		Password: "pass",
		Exp:      time.Now().Add(5 * time.Minute).Unix(),
	}
	tok, err := sso.Encrypt(payload, secret)
	require.NoError(t, err)
	return tok
}

func TestSSOLoginNoParam(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "GoblinFTP", rec.Body.String())
}

func TestSSOLoginDisabled(t *testing.T) {
	cfg := defaultTestConfig() // SSO disabled
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/?sso=anytoken", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, string(gftperrors.ErrUnauthorized), resp.Errors[0].Code)
}

func TestSSOLoginInvalidToken(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/?sso=garbage-token", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, string(gftperrors.ErrInvalidToken), resp.Errors[0].Code)
}

func TestSSOLoginExpiredToken(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	// Create an expired token
	payload := &sso.Payload{
		Type:     "ftp",
		Host:     "ftp.example.com",
		Port:     21,
		Username: "user",
		Password: "pass",
		Exp:      time.Now().Add(-5 * time.Minute).Unix(), // expired
	}
	tok, err := sso.Encrypt(payload, cfg.SSOSecret)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/?sso="+tok, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, string(gftperrors.ErrInvalidToken), resp.Errors[0].Code)
	assert.Contains(t, resp.Errors[0].Message, "expired")
}

func TestSSOLoginSuccess(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	tok := validSSO(t, cfg.SSOSecret)

	req := httptest.NewRequest(http.MethodGet, "/?sso="+tok, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/?", rec.Header().Get("Location"))

	// Check that a session cookie was set
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == api.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie, "expected gftp_session cookie to be set")
	assert.NotEmpty(t, sessionCookie.Value)
}

func TestSSOLoginReplay(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	tok := validSSO(t, cfg.SSOSecret)

	// First attempt should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/?sso="+tok, nil)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusFound, rec1.Code)

	// Second attempt with same token should fail
	req2 := httptest.NewRequest(http.MethodGet, "/?sso="+tok, nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusUnauthorized, rec2.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, string(gftperrors.ErrInvalidToken), resp.Errors[0].Code)
	assert.Contains(t, resp.Errors[0].Message, "already used")
}

func TestAuthStatusNoSession(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Connected      bool   `json:"connected"`
			SSOAutoConnect bool   `json:"ssoAutoConnect"`
			CSRFToken      string `json:"csrfToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.False(t, resp.Data.Connected)
	assert.False(t, resp.Data.SSOAutoConnect)
	assert.Empty(t, resp.Data.CSRFToken)
}

func TestAuthStatusWithSSOPending(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	tok := validSSO(t, cfg.SSOSecret)

	// Hit SSOLogin to create session with pending SSO
	loginReq := httptest.NewRequest(http.MethodGet, "/?sso="+tok, nil)
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusFound, loginRec.Code)

	// Get the session cookie
	cookies := loginRec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == api.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)

	// Now check auth status with that cookie
	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	statusReq.AddCookie(sessionCookie)
	statusRec := httptest.NewRecorder()
	e.ServeHTTP(statusRec, statusReq)

	assert.Equal(t, http.StatusOK, statusRec.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Connected      bool   `json:"connected"`
			SSOAutoConnect bool   `json:"ssoAutoConnect"`
			CSRFToken      string `json:"csrfToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.False(t, resp.Data.Connected)
	assert.True(t, resp.Data.SSOAutoConnect)
	assert.NotEmpty(t, resp.Data.CSRFToken)
}

func TestAuthStatusConnected(t *testing.T) {
	cfg := defaultTestConfig()
	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/home/user", nil },
		ChmodFn:      func(path string, mode uint32) error { return nil },
	}
	dialFn := func(protocol, addr, user, pass string, passive bool) (transfer.Client, error) {
		return mock, nil
	}
	e, store, _ := newTestApp(t, cfg, api.WithDial(dialFn))
	defer store.Close()

	// Connect via regular auth
	sess := connectAndGetSession(t, e)

	// Check auth status
	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	addSession(statusReq, sess)
	statusRec := httptest.NewRecorder()
	e.ServeHTTP(statusRec, statusReq)

	assert.Equal(t, http.StatusOK, statusRec.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Connected      bool   `json:"connected"`
			SSOAutoConnect bool   `json:"ssoAutoConnect"`
			CSRFToken      string `json:"csrfToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.True(t, resp.Data.Connected)
	assert.False(t, resp.Data.SSOAutoConnect)
	assert.NotEmpty(t, resp.Data.CSRFToken)
}

func TestSSOConnectNoPending(t *testing.T) {
	cfg := ssoEnabledConfig()
	e, store, _ := newTestApp(t, cfg)
	defer store.Close()

	// Create a plain session without sso_pending
	sess, err := store.New()
	require.NoError(t, err)
	csrfToken, err := auth.GenerateCSRFToken()
	require.NoError(t, err)
	sess.Data[auth.CSRFSessionKey] = csrfToken

	req := httptest.NewRequest(http.MethodPost, "/api/auth/sso-connect", nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: api.SessionCookieName, Value: sess.ID})
	req.Header.Set(auth.CSRFHeaderName, csrfToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, string(gftperrors.ErrUnauthorized), resp.Errors[0].Code)
}

func TestSSOConnectFullFlow(t *testing.T) {
	cfg := ssoEnabledConfig()
	mock := &testutil.MockClient{
		WorkingDirFn: func() (string, error) { return "/home/user", nil },
		ChmodFn:      func(path string, mode uint32) error { return nil },
	}
	dialFn := func(protocol, addr, user, pass string, passive bool) (transfer.Client, error) {
		return mock, nil
	}
	e, store, _ := newTestApp(t, cfg, api.WithDial(dialFn))
	defer store.Close()

	tok := validSSO(t, cfg.SSOSecret)

	// Step 1: SSOLogin
	loginReq := httptest.NewRequest(http.MethodGet, "/?sso="+tok, nil)
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusFound, loginRec.Code)

	// Get session cookie
	cookies := loginRec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == api.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)

	// Step 2: Get auth status to retrieve CSRF token
	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	statusReq.AddCookie(sessionCookie)
	statusRec := httptest.NewRecorder()
	e.ServeHTTP(statusRec, statusReq)
	require.Equal(t, http.StatusOK, statusRec.Code)

	var statusResp struct {
		Data struct {
			SSOAutoConnect bool   `json:"ssoAutoConnect"`
			CSRFToken      string `json:"csrfToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &statusResp))
	require.True(t, statusResp.Data.SSOAutoConnect)
	require.NotEmpty(t, statusResp.Data.CSRFToken)

	// Step 3: SSOConnect
	connectReq := httptest.NewRequest(http.MethodPost, "/api/auth/sso-connect", nil)
	connectReq.Header.Set("Content-Type", "application/json")
	connectReq.AddCookie(sessionCookie)
	connectReq.Header.Set(auth.CSRFHeaderName, statusResp.Data.CSRFToken)
	connectRec := httptest.NewRecorder()
	e.ServeHTTP(connectRec, connectReq)

	assert.Equal(t, http.StatusOK, connectRec.Code)
	var connectResp struct {
		Success bool `json:"success"`
		Data    struct {
			Capabilities struct {
				DisableChmod bool `json:"disableChmod"`
			} `json:"capabilities"`
			InitialDirectory string `json:"initialDirectory"`
			CSRFToken        string `json:"csrfToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(connectRec.Body.Bytes(), &connectResp))
	assert.True(t, connectResp.Success)
	assert.Equal(t, "/home/user", connectResp.Data.InitialDirectory)
	assert.NotEmpty(t, connectResp.Data.CSRFToken)
}
