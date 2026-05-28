// backend/internal/api/handler.go
package api

import (
	"github.com/darthsoup/goblinftp/internal/auth"
	"github.com/darthsoup/goblinftp/internal/config"
)

// Handler holds shared dependencies for all API request handlers.
type Handler struct {
	cfg      *config.Config
	store    *auth.Store
	throttle *auth.Throttle
}
