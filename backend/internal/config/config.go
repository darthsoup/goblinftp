package config

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

// UISettings maps the `ui` block of settings.json.
type UISettings struct {
	PageTitle             string  `json:"pageTitle"`
	ShowDotFiles          bool    `json:"showDotFiles"`
	ShowNavigationHistory bool    `json:"showNavigationHistory"`
	HelpURL               *string `json:"helpUrl"`
}

// EditorSettings maps the `editor` block.
type EditorSettings struct {
	OpenOnCreate      bool     `json:"openOnCreate"`
	AllowedExtensions []string `json:"allowedExtensions"`
	Disabled          bool     `json:"disabled"`
	ViewOnly          bool     `json:"viewOnly"`
}

// ConnectionSettings maps the `connection` block.
type ConnectionSettings struct {
	AllowedTypes          []string `json:"allowedTypes"`
	DisableChmod          bool     `json:"disableChmod"`
	RequestTimeoutSeconds int      `json:"requestTimeoutSeconds"`
}

// AccessSettings maps the `access` block.
type AccessSettings struct {
	AllowedClientAddresses []string `json:"allowedClientAddresses"`
	DeniedMessage          *string  `json:"deniedMessage"`
	PostLogoutURL          *string  `json:"postLogoutUrl"`
}

// Settings mirrors settings.json; used for runtime-configurable UI/editor/connection/access settings.
type Settings struct {
	Language   string             `json:"language"`
	UI         UISettings         `json:"ui"`
	Editor     EditorSettings     `json:"editor"`
	Connection ConnectionSettings `json:"connection"`
	Access     AccessSettings     `json:"access"`
}

// Config holds all runtime configuration for GoblinFTP.
type Config struct {
	Port                  string
	LogLevel              string
	SessionSecret         []byte
	DownloadTokenSecret   []byte
	SSOEnabled            bool
	SSOSecret             []byte
	ChunkSize             int64
	LoginMaxAttempts      int
	LoginCooldownSeconds  int
	SessionTTLSeconds     int
	SentryDSN             string
	LoginDisabledRedirect string
	Settings              Settings
}

func defaultSettings() Settings {
	return Settings{
		Language: "en",
		UI: UISettings{
			PageTitle:             "GoblinFTP",
			ShowDotFiles:          false,
			ShowNavigationHistory: true,
		},
		Editor: EditorSettings{
			OpenOnCreate:      false,
			AllowedExtensions: []string{"txt", "htm", "html", "php", "js", "css", "xml", "json", "py", "rb", "go", "sh"},
			Disabled:          false,
			ViewOnly:          false,
		},
		Connection: ConnectionSettings{
			AllowedTypes:          []string{"ftp", "sftp"},
			DisableChmod:          false,
			RequestTimeoutSeconds: 30,
		},
		Access: AccessSettings{
			AllowedClientAddresses: []string{},
		},
	}
}

// Load reads configuration from environment variables and an optional settings.json file.
// If settingsPath is empty or the file does not exist, settings.json defaults are used.
// Auto-generates SESSION_SECRET and DOWNLOAD_TOKEN_SECRET if not set, logging a warning.
func Load(logger *slog.Logger, settingsPath string) (*Config, error) {
	cfg := &Config{
		Port:                  envOr("GFTP_PORT", "8080"),
		LogLevel:              envOr("GFTP_LOG_LEVEL", "info"),
		ChunkSize:             5 * 1024 * 1024,
		LoginMaxAttempts:      5,
		LoginCooldownSeconds:  300,
		SessionTTLSeconds:     7200,
		SentryDSN:             os.Getenv("GFTP_SENTRY_DSN"),
		LoginDisabledRedirect: os.Getenv("GFTP_LOGIN_DISABLED_REDIRECT"),
		Settings:              defaultSettings(),
	}

	cfg.SSOEnabled = os.Getenv("GFTP_SSO_ENABLED") == "true"
	if raw := os.Getenv("GFTP_SSO_SECRET"); raw != "" {
		cfg.SSOSecret = []byte(raw)
	}
	if cfg.SSOEnabled && len(cfg.SSOSecret) == 0 {
		return nil, fmt.Errorf("GFTP_SSO_SECRET must be set when GFTP_SSO_ENABLED is true")
	}

	if raw := os.Getenv("GFTP_CHUNK_SIZE"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid GFTP_CHUNK_SIZE: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("invalid GFTP_CHUNK_SIZE: must be positive, got %d", n)
		}
		cfg.ChunkSize = n
	}

	if raw := os.Getenv("GFTP_LOGIN_MAX_ATTEMPTS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid GFTP_LOGIN_MAX_ATTEMPTS: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("invalid GFTP_LOGIN_MAX_ATTEMPTS: must be positive, got %d", n)
		}
		cfg.LoginMaxAttempts = n
	}

	if raw := os.Getenv("GFTP_LOGIN_COOLDOWN_SECS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid GFTP_LOGIN_COOLDOWN_SECS: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("invalid GFTP_LOGIN_COOLDOWN_SECS: must be positive, got %d", n)
		}
		cfg.LoginCooldownSeconds = n
	}

	if raw := os.Getenv("GFTP_SESSION_TTL_SECS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid GFTP_SESSION_TTL_SECS: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("invalid GFTP_SESSION_TTL_SECS: must be positive, got %d", n)
		}
		cfg.SessionTTLSeconds = n
	}

	if raw := os.Getenv("GFTP_SESSION_SECRET"); raw != "" {
		cfg.SessionSecret = []byte(raw)
	} else {
		secret, err := generateSecret(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate session secret: %w", err)
		}
		cfg.SessionSecret = secret
		if logger != nil {
			logger.Warn("GFTP_SESSION_SECRET not set, using ephemeral random secret — sessions will be invalidated on restart")
		}
	}

	if raw := os.Getenv("GFTP_DOWNLOAD_TOKEN_SECRET"); raw != "" {
		cfg.DownloadTokenSecret = []byte(raw)
	} else {
		secret, err := generateSecret(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate download token secret: %w", err)
		}
		cfg.DownloadTokenSecret = secret
		if logger != nil {
			logger.Warn("GFTP_DOWNLOAD_TOKEN_SECRET not set, using ephemeral random secret — download links will be invalidated on restart")
		}
	}

	if settingsPath != "" {
		data, err := os.ReadFile(settingsPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read settings file %q: %w", settingsPath, err)
		}
		if err == nil {
			if jsonErr := json.Unmarshal(data, &cfg.Settings); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse settings file %q: %w", settingsPath, jsonErr)
			}
		}
	}

	if title := os.Getenv("GFTP_PAGE_TITLE"); title != "" {
		cfg.Settings.UI.PageTitle = title
	}

	return cfg, nil
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func generateSecret(length int) ([]byte, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
