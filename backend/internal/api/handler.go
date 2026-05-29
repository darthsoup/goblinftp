// backend/internal/api/handler.go
package api

import (
	"github.com/darthsoup/goblinftp/internal/auth"
	"github.com/darthsoup/goblinftp/internal/config"
	"github.com/darthsoup/goblinftp/internal/sso"
	"github.com/darthsoup/goblinftp/internal/transfer"
)

// DialFunc creates a transfer.Client for the given protocol, address, credentials, and passive flag.
type DialFunc func(protocol, addr, user, pass string, passive bool) (transfer.Client, error)

// HandlerOption is a functional option for constructing a Handler.
type HandlerOption func(*Handler)

// WithDial overrides the dial function (primarily for testing).
func WithDial(fn DialFunc) HandlerOption {
	return func(h *Handler) {
		h.dial = fn
	}
}

// Handler holds shared dependencies for all API handlers.
type Handler struct {
	cfg      *config.Config
	store    *auth.Store
	throttle *auth.Throttle
	dataDir  string
	dial     DialFunc
	ssoUsed  *sso.UsedSet
}

func newHandler(cfg *config.Config, store *auth.Store, thr *auth.Throttle, opts []HandlerOption) *Handler {
	h := &Handler{
		cfg:      cfg,
		store:    store,
		throttle: thr,
		dataDir:  cfg.DataDir,
		dial:     defaultDial,
		ssoUsed:  sso.NewUsedSet(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
