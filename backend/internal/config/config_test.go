package config_test

import (
	"os"
	"testing"

	"github.com/darthsoup/goblinftp/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GFTP_PORT", "GFTP_LOG_LEVEL", "GFTP_SESSION_SECRET", "GFTP_DOWNLOAD_TOKEN_SECRET",
		"GFTP_SSO_ENABLED", "GFTP_SSO_SECRET", "GFTP_CHUNK_SIZE",
		"GFTP_LOGIN_MAX_ATTEMPTS", "GFTP_LOGIN_COOLDOWN_SECS", "GFTP_SESSION_TTL_SECS",
		"GFTP_SENTRY_DSN", "GFTP_PAGE_TITLE", "GFTP_LOGIN_DISABLED_REDIRECT", "GFTP_SETTINGS_PATH",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := config.Load(nil, "")
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.NotEmpty(t, cfg.SessionSecret)
	assert.NotEmpty(t, cfg.DownloadTokenSecret)
	assert.Equal(t, int64(5*1024*1024), cfg.ChunkSize)
	assert.Equal(t, 5, cfg.LoginMaxAttempts)
	assert.Equal(t, 300, cfg.LoginCooldownSeconds)
	assert.Equal(t, 7200, cfg.SessionTTLSeconds)
	assert.False(t, cfg.SSOEnabled)

	assert.Equal(t, "GoblinFTP", cfg.Settings.UI.PageTitle)
	assert.Equal(t, []string{"ftp", "sftp"}, cfg.Settings.Connection.AllowedTypes)
	assert.Equal(t, "en", cfg.Settings.Language)
	assert.False(t, cfg.Settings.Connection.DisableChmod)
	assert.Equal(t, 30, cfg.Settings.Connection.RequestTimeoutSeconds)
}

func TestLoadFromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_PORT", "9090")
	t.Setenv("GFTP_LOG_LEVEL", "debug")
	t.Setenv("GFTP_SESSION_SECRET", "my-session-secret")
	t.Setenv("GFTP_DOWNLOAD_TOKEN_SECRET", "my-token-secret")
	t.Setenv("GFTP_SSO_ENABLED", "true")
	t.Setenv("GFTP_SSO_SECRET", "sso-secret")
	t.Setenv("GFTP_CHUNK_SIZE", "1048576")
	t.Setenv("GFTP_LOGIN_MAX_ATTEMPTS", "3")
	t.Setenv("GFTP_LOGIN_COOLDOWN_SECS", "60")
	t.Setenv("GFTP_SESSION_TTL_SECS", "3600")
	t.Setenv("GFTP_PAGE_TITLE", "MyFTP")

	cfg, err := config.Load(nil, "")
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, []byte("my-session-secret"), cfg.SessionSecret)
	assert.Equal(t, []byte("my-token-secret"), cfg.DownloadTokenSecret)
	assert.True(t, cfg.SSOEnabled)
	assert.Equal(t, int64(1048576), cfg.ChunkSize)
	assert.Equal(t, 3, cfg.LoginMaxAttempts)
	assert.Equal(t, 60, cfg.LoginCooldownSeconds)
	assert.Equal(t, 3600, cfg.SessionTTLSeconds)
	assert.Equal(t, "MyFTP", cfg.Settings.UI.PageTitle)
}

func TestLoadSettingsJSON(t *testing.T) {
	clearEnv(t)
	content := `{
		"language":"de",
		"ui":{"pageTitle":"Test FTP","showDotFiles":true,"showNavigationHistory":false,"helpUrl":null},
		"editor":{"openOnCreate":false,"allowedExtensions":["txt"],"disabled":true,"viewOnly":false},
		"connection":{"allowedTypes":["ftp"],"disableChmod":true,"requestTimeoutSeconds":60},
		"access":{"allowedClientAddresses":["127.0.0.1"],"deniedMessage":null,"postLogoutUrl":null}
	}`
	f, err := os.CreateTemp(".", "settings*.json")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	cfg, err := config.Load(nil, f.Name())
	require.NoError(t, err)

	assert.Equal(t, "de", cfg.Settings.Language)
	assert.Equal(t, "Test FTP", cfg.Settings.UI.PageTitle)
	assert.True(t, cfg.Settings.UI.ShowDotFiles)
	assert.True(t, cfg.Settings.Editor.Disabled)
	assert.Equal(t, []string{"ftp"}, cfg.Settings.Connection.AllowedTypes)
	assert.True(t, cfg.Settings.Connection.DisableChmod)
	assert.Equal(t, []string{"127.0.0.1"}, cfg.Settings.Access.AllowedClientAddresses)
}

func TestLoadPageTitleEnvOverridesSettings(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_PAGE_TITLE", "Override Title")

	content := `{"language":"en","ui":{"pageTitle":"From File","showDotFiles":false,"showNavigationHistory":true,"helpUrl":null},"editor":{"openOnCreate":false,"allowedExtensions":[],"disabled":false,"viewOnly":false},"connection":{"allowedTypes":["ftp","sftp"],"disableChmod":false,"requestTimeoutSeconds":30},"access":{"allowedClientAddresses":[],"deniedMessage":null,"postLogoutUrl":null}}`
	f, err := os.CreateTemp(".", "settings*.json")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	cfg, err := config.Load(nil, f.Name())
	require.NoError(t, err)
	assert.Equal(t, "Override Title", cfg.Settings.UI.PageTitle)
}

func TestLoadAutoGeneratesUniqueSecrets(t *testing.T) {
	clearEnv(t)
	cfg1, err := config.Load(nil, "")
	require.NoError(t, err)
	cfg2, err := config.Load(nil, "")
	require.NoError(t, err)

	assert.NotEqual(t, cfg1.SessionSecret, cfg2.SessionSecret)
	assert.NotEqual(t, cfg1.DownloadTokenSecret, cfg2.DownloadTokenSecret)
}

func TestLoadInvalidSettingsJSON(t *testing.T) {
	clearEnv(t)
	f, err := os.CreateTemp(".", "settings*.json")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.WriteString("not json")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, err = config.Load(nil, f.Name())
	assert.Error(t, err)
}

func TestLoadInvalidChunkSize(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_CHUNK_SIZE", "notanumber")
	_, err := config.Load(nil, "")
	assert.Error(t, err)
}

func TestLoadRejectsNonPositiveChunkSize(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_CHUNK_SIZE", "0")
	_, err := config.Load(nil, "")
	assert.Error(t, err)
}

func TestLoadInvalidLoginMaxAttempts(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_LOGIN_MAX_ATTEMPTS", "0")
	_, err := config.Load(nil, "")
	assert.Error(t, err)
}

func TestLoadInvalidLoginCooldownSeconds(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_LOGIN_COOLDOWN_SECS", "0")
	_, err := config.Load(nil, "")
	assert.Error(t, err)
}

func TestLoadInvalidSessionTTL(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_SESSION_TTL_SECS", "-1")
	_, err := config.Load(nil, "")
	assert.Error(t, err)
}

func TestLoadSSOEnabledWithoutSecretIsError(t *testing.T) {
	clearEnv(t)
	t.Setenv("GFTP_SSO_ENABLED", "true")
	_, err := config.Load(nil, "")
	assert.Error(t, err)
}

func TestLoadMissingSettingsFileIsNotAnError(t *testing.T) {
	clearEnv(t)
	cfg, err := config.Load(nil, "./does-not-exist/settings.json")
	require.NoError(t, err)
	assert.Equal(t, "GoblinFTP", cfg.Settings.UI.PageTitle)
}
