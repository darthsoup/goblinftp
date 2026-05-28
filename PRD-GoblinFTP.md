# Product Requirements Document
## GoblinFTP (GFTP)

**Version:** 0.4
**Status:** For review
**Repository:** `darthsoup/goblinftp` (https://github.com/darthsoup/goblinftp)

---

## 1. Overview

GoblinFTP (GFTP) is a self-hosted, web-based FTP/SFTP client. Users deploy it as a Docker container and access it via browser to manage files on remote FTP or SFTP servers. The application consists of a Go backend API that proxies FTP/SFTP protocol operations, and a Nuxt 3 frontend SPA.

This is a clean rewrite of Monsta FTP v2.14.x, modernising the tech stack while delivering full feature parity. GoblinFTP is fully open-source with no licence gating.

**Backend:** Go  
**Frontend:** Nuxt 3 (SPA mode) · Nuxt UI v3 · Tailwind CSS v4  
**Deployment:** Docker container  

---

## 2. Goals

- Go backend with `goftp/ftp` (FTP) and `golang.org/x/crypto/ssh` + `pkg/sftp` (SFTP)
- Nuxt 3 SPA frontend using Nuxt UI + Tailwind CSS
- Docker container as the primary deployment target
- Full feature parity with Monsta FTP v2.14.x
- All features available to all users — no licence gating
- SSO-capable login links (external systems can embed GoblinFTP via signed tokens)
- Sentry integration for error tracking
- Clean, testable codebase

## 3. Non-Goals

- Backward-compatible configuration format
- Supporting non-containerised deployments as a first-class target
- Saved authentication profiles (removed for GDPR/security reasons)
- Diagnostics tools endpoint
- Self-update installer

---

## 4. Deployment Model

GoblinFTP ships as a single Docker image. Configuration is via environment variables.

```bash
# Minimal run
docker run -p 8080:80 goblintools/gftp

# With custom config
docker run -p 8080:80 \
  -e GFTP_PAGE_TITLE="My FTP Client" \
  -e GFTP_SSO_SECRET="your-shared-secret" \
  -e GFTP_SENTRY_DSN="https://..." \
  -v ./settings.json:/app/settings.json:ro \
  goblintools/gftp
```

**docker-compose.yml:**
```yaml
services:
  gftp:
    image: goblintools/gftp
    ports:
      - "8080:80"
    environment:
      GFTP_PAGE_TITLE: "My FTP Client"
      GFTP_SSO_SECRET: "${GFTP_SSO_SECRET}"
      GFTP_SENTRY_DSN: "${GFTP_SENTRY_DSN}"
    volumes:
      - ./settings.json:/app/settings.json:ro
```

The container is **fully stateless**. `/app/data` (temp upload/download files) is ephemeral — no volume mount is required. Nothing is persisted between container restarts.

---

## 5. Backend: Go

| Concern | Library |
|---------|---------|
| HTTP framework | Echo or Chi |
| FTP | `github.com/goftp/ftp` |
| SFTP | `golang.org/x/crypto/ssh` + `github.com/pkg/sftp` |
| Configuration | `github.com/caarlos0/env` |
| Logging | `log/slog` (stdlib, Go 1.21+) |
| Error tracking | `github.com/getsentry/sentry-go` |
| Encryption | stdlib `crypto/aes`, `crypto/hmac` |
| Testing | stdlib `testing` + `github.com/stretchr/testify` |
| Archive | `archive/zip` (stdlib) + `github.com/mholt/archiver` for tar/bz2 |

---

## 6. Configuration

Configuration follows Go conventions: a typed `Config` struct populated from environment variables using struct tags. No PHP-style config files. A single `config.Load()` function parses all values at startup and fails fast on missing required fields.

### Config struct (illustrative)

```go
type Config struct {
    Server  ServerConfig
    Auth    AuthConfig
    SSO     SSOConfig
    Log     LogConfig
    Sentry  SentryConfig
    FTP     FTPConfig
    Storage StorageConfig
}

type ServerConfig struct {
    Port          int           `env:"GFTP_PORT"           envDefault:"8080"`
    PageTitle     string        `env:"GFTP_PAGE_TITLE"     envDefault:"GoblinFTP"`
    MaxFileSize   string        `env:"GFTP_MAX_FILE_SIZE"  envDefault:"2G"`
    ChunkSize     string        `env:"GFTP_CHUNK_SIZE"     envDefault:"default"`
    Timezone      string        `env:"GFTP_TIMEZONE"       envDefault:"UTC"`
    DownloadSecret string       `env:"GFTP_DOWNLOAD_TOKEN_SECRET,required"`
}

type AuthConfig struct {
    MaxLoginFailures    int           `env:"GFTP_MAX_LOGIN_FAILURES"         envDefault:"5"`
    LoginFailuresReset  time.Duration `env:"GFTP_LOGIN_FAILURES_RESET"       envDefault:"5m"`
    DisableFailureBan   bool          `env:"GFTP_DISABLE_LOGIN_FAILURE_BAN"  envDefault:"false"`
    SessionSecret       string        `env:"GFTP_SESSION_SECRET,required"`
    SessionLifetime     time.Duration `env:"GFTP_SESSION_LIFETIME"           envDefault:"24h"`
    RememberMeLifetime  time.Duration `env:"GFTP_REMEMBER_ME_LIFETIME"       envDefault:"720h"` // 30 days
}

type SSOConfig struct {
    Enabled bool          `env:"GFTP_SSO_ENABLED"  envDefault:"false"`
    Secret  string        `env:"GFTP_SSO_SECRET"`
    TTL     time.Duration `env:"GFTP_SSO_TTL"      envDefault:"15m"`
}

type LogConfig struct {
    Level  string `env:"GFTP_LOG_LEVEL"   envDefault:"info"`   // debug|info|warn|error
    Format string `env:"GFTP_LOG_FORMAT"  envDefault:"json"`   // json|text
}

type SentryConfig struct {
    DSN         string  `env:"GFTP_SENTRY_DSN"`
    Environment string  `env:"GFTP_SENTRY_ENVIRONMENT"   envDefault:"production"`
    Release     string  `env:"GFTP_SENTRY_RELEASE"`
    SampleRate  float64 `env:"GFTP_SENTRY_SAMPLE_RATE"   envDefault:"1.0"`
}

type FTPConfig struct {
    SSHAgentAuthEnabled bool `env:"GFTP_SSH_AGENT_AUTH_ENABLED"  envDefault:"false"`
    SSHKeyAuthEnabled   bool `env:"GFTP_SSH_KEY_AUTH_ENABLED"    envDefault:"false"`
    MaxConcurrentUploads int `env:"GFTP_MAX_CONCURRENT_UPLOADS"  envDefault:"10"`
    UploadSocketTimeout  time.Duration `env:"GFTP_UPLOAD_SOCKET_TIMEOUT" envDefault:"5m"`
}

type StorageConfig struct {
    DataDir    string `env:"GFTP_DATA_DIR"    envDefault:"/app/data"`
    StorageDir string `env:"GFTP_STORAGE_DIR" envDefault:"/app/storage"`
}
```

### settings.json (user-facing, optional volume mount)

Runtime UI settings. Loaded at startup; writable via the settings API. All keys are optional — defaults shown below.

```json
{
  "language": "en",

  "ui": {
    "pageTitle": "GoblinFTP",
    "showDotFiles": false,
    "showNavigationHistory": true,
    "helpUrl": null
  },

  "editor": {
    "openOnCreate": false,
    "allowedExtensions": ["txt", "htm", "html", "php", "js", "css", "xml", "json", "py", "rb", "go", "sh"],
    "disabled": false,
    "viewOnly": false
  },

  "connection": {
    "allowedTypes": ["ftp", "sftp"],
    "disableChmod": false,
    "requestTimeoutSeconds": 30
  },

  "access": {
    "allowedClientAddresses": [],
    "deniedMessage": null,
    "postLogoutUrl": null
  }
}
```

**Key changes from Monsta FTP `settings.json`:**

| Old key | New key | Notes |
|---------|---------|-------|
| `editableFileExtensions` (string) | `editor.allowedExtensions` (array) | Array instead of comma-separated string |
| `editNewFilesImmediately` | `editor.openOnCreate` | |
| `disableFileEdit` | `editor.disabled` | |
| `disableFileView` | `editor.viewOnly` | |
| `showDotFiles` | `ui.showDotFiles` | |
| `hideHistoryBar` | `ui.showNavigationHistory` | Inverted meaning |
| `helpUrl` | `ui.helpUrl` | |
| `connectionRestrictions.types` | `connection.allowedTypes` | |
| `disableChmod` | `connection.disableChmod` | |
| `xhrTimeoutSeconds` | `connection.requestTimeoutSeconds` | |
| `allowedClientAddresses` | `access.allowedClientAddresses` | |
| `disallowedClientMessage` | `access.deniedMessage` | |
| `postLogoutUrl` | `access.postLogoutUrl` | |

---

## 7. Justfile

A `justfile` at the repository root covers all common developer tasks.

```just
# ── Development ────────────────────────────────────────────────────────────────
dev:          # Start frontend + backend (overmind / process-compose)
dev-fe:       # cd frontend && nuxt dev
dev-be:       # go run ./cmd/gftp

# ── Build ──────────────────────────────────────────────────────────────────────
build:        # build-fe + build-be
build-fe:     # cd frontend && nuxt generate  → .output/public/
build-be:     # go build -o bin/gftp ./cmd/gftp

# ── Test ───────────────────────────────────────────────────────────────────────
test:         # test-fe + test-be
test-fe:      # cd frontend && vitest run
test-be:      # go test ./...

# ── Lint / Format ──────────────────────────────────────────────────────────────
lint:         # lint-fe + lint-be
lint-fe:      # eslint frontend/ + nuxi typecheck
lint-be:      # golangci-lint run ./...
fmt:          # cd frontend && prettier --write .  &&  gofmt -w .

# ── Docker ─────────────────────────────────────────────────────────────────────
docker-build: # docker build -t goblintools/gftp .
docker-run:   # docker run -p 8080:80 goblintools/gftp
docker-push:  # docker push goblintools/gftp
docker-up:    # docker compose up --build
docker-down:  # docker compose down

# ── Utilities ──────────────────────────────────────────────────────────────────
i18n-check:   # Report keys in en.json missing from de.json
clean:        # rm -rf frontend/.output frontend/node_modules bin/
```

`just dev` uses **overmind** (or process-compose) to run both servers with unified log output.

---

## 8. Feature Requirements

### 8.1 Authentication & Session

| ID | Requirement |
|----|-------------|
| AUTH-1 | FTP login: host, port, username, password, passive mode toggle, SSL toggle |
| AUTH-2 | SFTP login: host, port, username + one of: password, public key file (+ optional passphrase), SSH agent |
| AUTH-3 | Login failure throttling: configurable max attempts and cooldown per host+user |
| AUTH-4 | "Keep me logged in" — extends session cookie to 30 days (configurable via `GFTP_REMEMBER_ME_LIFETIME`) |
| AUTH-5 | Session fixation protection: regenerate session ID on successful login |
| AUTH-6 | CSRF protection on all state-changing API endpoints |
| AUTH-8 | SSO login link — see §8.1.1 |
| AUTH-9 | Active connection stored in session; credentials not re-sent per request |
| AUTH-10 | Configurable option to disable the login form entirely (external auth / SSO-only mode) |

#### 8.1.1 SSO Login Link (AUTH-8)

An external system (hosting panel, CMS, identity provider) generates a signed, encrypted token and embeds it in a GoblinFTP URL. On page load, GoblinFTP decrypts the token server-side and auto-connects — no user interaction required.

> **Note — this is NOT JWT.** Standard JWT (JWS) is only *signed*, not encrypted; the payload is visible in base64. GoblinFTP's SSO token is *fully encrypted* using AES-256-GCM — the payload is opaque without the shared secret. The format is conceptually similar to the content layer of JWE (RFC 7516) but uses a lightweight custom wire format: `iv (12 B) | auth_tag (16 B) | ciphertext (N B)`, base64url-encoded.

**Token design:**

```
URL:  https://gftp.example.com/?sso=<token>

Token: AES-256-GCM encrypted JSON payload, base64url-encoded.
       Key derived from GFTP_SSO_SECRET using HKDF-SHA256 with info="gftp-sso".
       Wire format: iv (12 bytes) | auth_tag (16 bytes) | ciphertext

Plaintext payload (before encryption):
{
  "type":             "sftp",         // "ftp" | "sftp"
  "host":             "files.example.com",
  "port":             22,
  "username":         "user",
  "password":         "secret",       // NEVER appears in the URL
  "initialDirectory": "/home/user",
  "language":         "de",           // optional — overrides browser/settings default
  "exp":              1716858000      // Unix timestamp; token rejected after this
}
```

**Server-side flow:**
1. GoblinFTP receives `GET /?sso=<token>`
2. Backend decrypts and validates the token (checks `exp`, verifies integrity)
3. If valid: marks token as used (in-memory set — single-instance), creates a pre-auth session, redirects to `/?` (clean URL — token removed from browser history)
4. Frontend auto-connects using credentials stored in the server-side session
5. Credentials are never exposed to the frontend JavaScript or browser history

**Security constraints:**
- Token TTL: default 15 minutes, configurable via `GFTP_SSO_TTL`
- Expired or tampered tokens return a generic error (no detail leaked)
- SSO is opt-in; disabled unless `GFTP_SSO_ENABLED=true` and `GFTP_SSO_SECRET` is set
- One-time use: token is invalidated server-side after first successful use (replay protection)

**Reference implementation — PHP / Laravel:**

```php
// app/Services/GoblinFtpSsoService.php

namespace App\Services;

class GoblinFtpSsoService
{
    public function generateLoginUrl(
        string  $type,
        string  $host,
        int     $port,
        string  $username,
        string  $password,
        string  $initialDirectory = '/',
        ?string $language = null,
        int     $ttlSeconds = 900,
    ): string {
        $payload = array_filter([
            'type'             => $type,
            'host'             => $host,
            'port'             => $port,
            'username'         => $username,
            'password'         => $password,
            'initialDirectory' => $initialDirectory,
            'language'         => $language,
            'exp'              => time() + $ttlSeconds,
        ], fn($v) => $v !== null);

        $token = $this->encryptPayload($payload);

        return config('services.goblinftp.url') . '/?sso=' . $token;
    }

    private function encryptPayload(array $payload): string
    {
        // Derive 32-byte key from the shared secret
        $key = hash_hkdf('sha256', config('services.goblinftp.sso_secret'), 32, 'gftp-sso');

        $iv  = random_bytes(12);   // 96-bit GCM nonce
        $tag = '';

        $ciphertext = openssl_encrypt(
            json_encode($payload, JSON_THROW_ON_ERROR),
            'aes-256-gcm',
            $key,
            OPENSSL_RAW_DATA,
            $iv,
            $tag,
            '',
            16                     // 128-bit auth tag
        );

        // Wire format: iv (12 B) | auth_tag (16 B) | ciphertext (N B)
        return rtrim(strtr(base64_encode($iv . $tag . $ciphertext), '+/', '-_'), '=');
    }
}
```

```php
// config/services.php  — add to the existing array
'goblinftp' => [
    'url'        => env('GOBLINFTP_URL', 'https://ftp.example.com'),
    'sso_secret' => env('GOBLINFTP_SSO_SECRET'),
],
```

```php
// Example usage in a controller
$url = app(GoblinFtpSsoService::class)->generateLoginUrl(
    type:             'sftp',
    host:             $user->ftp_host,
    port:             22,
    username:         $user->ftp_username,
    password:         decrypt($user->ftp_password_encrypted),
    initialDirectory: '/home/' . $user->ftp_username,
    language:         app()->getLocale(),   // pass the current app locale
);

return redirect($url);
```

### 8.2 File Browser

| ID | Requirement |
|----|-------------|
| BROWSE-1 | List directory: name, size, modification date, permissions (octal), file/folder type icon |
| BROWSE-2 | Sortable columns: name, size, modified date |
| BROWSE-3 | Configurable column visibility: icon, name, size, modified, permissions, properties |
| BROWSE-4 | Show/hide dot files (e.g. `.htaccess`) — user-settable, off by default |
| BROWSE-5 | Back/forward navigation history within the session |
| BROWSE-6 | Breadcrumb bar (path history, collapsible) |
| BROWSE-7 | `..` parent directory entry |
| BROWSE-8 | Context menu per file row (see §8.4) |
| BROWSE-9 | Multi-select via mouse drag or click |

### 8.3 File Transfer

| ID | Requirement |
|----|-------------|
| XFER-1 | Upload single or multiple files via file picker |
| XFER-2 | Drag-and-drop upload onto the file browser |
| XFER-3 | Chunked upload for large files (configurable chunk size; auto-detected from server limits) |
| XFER-4 | Upload retry with exponential backoff on transient errors (up to 5 attempts) |
| XFER-5 | Upload to a new subdirectory in one operation (create dir then upload) |
| XFER-6 | Transfer progress UI: filename, bytes transferred / total, transfer rate, `N of M` counter |
| XFER-7 | Cancel individual upload or cancel all in-progress uploads |
| XFER-8 | Download single file |
| XFER-9 | Download multiple selected files as a ZIP (assembled server-side) |
| XFER-10 | Configurable max concurrent uploads (`GFTP_MAX_CONCURRENT_UPLOADS`) |

### 8.4 File Operations

| ID | Requirement |
|----|-------------|
| OPS-1 | Create new file (with optional immediate open in editor) |
| OPS-2 | Create new folder |
| OPS-3 | Rename file or folder |
| OPS-4 | Move (rename to a different path) |
| OPS-5 | Copy file or folder (duplicate on remote) |
| OPS-6 | Cut / Copy / Paste clipboard (in-browser; sends copy or rename to API on paste) |
| OPS-7 | Delete single item with confirmation |
| OPS-8 | Delete multiple selected items |
| OPS-9 | CHMOD — visual permission builder (owner/group/other × rwx checkboxes) + octal display |
| OPS-10 | Extract archive (ZIP, tar.gz, tar.bz2) in-place on server |
| OPS-11 | Create ZIP archive from selected items (assembled server-side, uploaded to current dir) |
| OPS-12 | Copy filename to clipboard |
| OPS-13 | File properties panel: full path, size, modified date, permissions |

### 8.5 File Editor

| ID | Requirement |
|----|-------------|
| EDIT-1 | Open text files in an embedded Monaco (VS Code) editor |
| EDIT-2 | Configurable list of editable file extensions |
| EDIT-3 | Read-only "view" mode when editing is disabled |
| EDIT-4 | Multi-tab editor — open multiple files, switch between them |
| EDIT-5 | Unsaved-change indicator on tab |
| EDIT-6 | Save file (overwrite on remote) |
| EDIT-7 | Auto-save toggle |
| EDIT-8 | Configurable line separator (LF / CRLF / CR) |

### 8.6 Internationalisation

| ID | Requirement |
|----|-------------|
| I18N-1 | Ship two locales: **English (en)** and **German (de)**. Default is English. |
| I18N-2 | Language detection priority: SSO token `language` field → browser `Accept-Language` header → `settings.json` default → `en` |
| I18N-3 | Localised error messages: backend returns error keys + context; frontend resolves to strings |
| I18N-4 | Language JSON files live in `frontend/i18n/locales/en.json` and `de.json` |

### 8.7 Settings & Configuration

See §6 for the full config reference. Settings exposed to the UI (via `settings.json`) are documented there.

### 8.8 Action Logging

| ID | Requirement |
|----|-------------|
| LOG-1 | Log the following actions: login, logout, upload, download, delete, rename, move, CHMOD, create folder |
| LOG-2 | Each entry includes: timestamp, log level, action name, connection host, remote path, username, outcome |
| LOG-3 | **Passwords, private key contents, and SSO token payloads are NEVER included in any log output** |
| LOG-4 | Log format follows `log/slog` conventions (structured JSON by default; `text` for dev) |
| LOG-5 | Log level controlled by `GFTP_LOG_LEVEL` (debug / info / warn / error) |
| LOG-6 | Logs written to stdout (12-factor; Docker collects them) |

**Credential scrubbing rule:** Any struct or map passed to a log call must have password fields zeroed or replaced with `"[REDACTED]"` before logging. A `SafeLogAttrs(conn Connection) []slog.Attr` helper function will produce log-safe attributes from a connection struct.

---

## 9. Observability: Sentry

GoblinFTP ships with Sentry integration for both backend and frontend.

### Backend (`sentry-go`)

- Initialised at startup if `GFTP_SENTRY_DSN` is set; skipped silently if not
- All unhandled panics and 5xx errors are captured automatically via Echo/Chi middleware
- `sentry.WithScope` used to attach request context (user session ID, action name) — never the connection password
- Release version set from `GFTP_SENTRY_RELEASE` (populated at Docker build time via build args)

### Frontend (`@sentry/nuxtjs`)

- Initialised via `nuxt.config.ts` if `NUXT_PUBLIC_SENTRY_DSN` is set
- Captures unhandled JS exceptions and API error responses
- Source maps uploaded at build time for readable stack traces
- User PII (usernames, hostnames) are scrubbed from breadcrumbs by default

### Configuration

| Variable | Purpose |
|----------|---------|
| `GFTP_SENTRY_DSN` | Backend Sentry DSN (optional) |
| `GFTP_SENTRY_ENVIRONMENT` | Environment tag (default: `production`) |
| `GFTP_SENTRY_RELEASE` | Release identifier (e.g. `gftp@1.2.3`) |
| `GFTP_SENTRY_SAMPLE_RATE` | Error sample rate 0.0–1.0 (default: `1.0`) |
| `NUXT_PUBLIC_SENTRY_DSN` | Frontend Sentry DSN (optional, separate project recommended) |

---

## 10. Security Requirements

| ID | Requirement |
|----|-------------|
| SEC-1 | CSRF token required on all state-changing API requests |
| SEC-2 | Content-Security-Policy header on all HTML responses |
| SEC-3 | Session ID regenerated on login |
| SEC-4 | Login failure throttling (AUTH-3) |
| SEC-5 | All user-supplied data escaped before rendering |
| SEC-6 | SSO tokens encrypted with AES-256-GCM; one-time use enforced server-side |
| SEC-7 | Download tokens signed with HMAC-SHA256 |
| SEC-8 | IP allowlist enforcement (`allowedClientAddresses` in settings.json) |
| SEC-9 | Passwords and credentials never written to logs (LOG-3) |
| SEC-10 | Sentry integration must not transmit passwords or private key material |

---

## 11. Non-Functional Requirements

| Category | Requirement |
|----------|-------------|
| **Performance** | Directory listing of 1,000 items renders in < 500 ms on local network |
| **Container build** | `docker build` only; no external CI services required |
| **Container size** | Target < 60 MB final image |
| **Startup time** | Container ready to serve within 3 s |
| **Accessibility** | All interactive controls have ARIA labels; modals are keyboard-navigable |
| **Mobile** | Responsive layout; core operations usable on phone browsers |
| **Browser support** | Last 2 versions of Chrome, Firefox, Safari, Edge |
| **Test coverage** | Backend ≥ 70% line coverage on business logic; frontend Pinia stores fully unit-tested |

---

## 12. System Architecture

### 12.1 Repository Layout

```
gftp/
├── frontend/                   # Nuxt 3 SPA
│   ├── app.vue
│   ├── components/             # Nuxt UI + custom components
│   ├── composables/            # useFileBrowser, useTransfer, useEditor …
│   ├── stores/                 # Pinia: auth, files, transfer, editor, settings
│   ├── pages/index.vue         # Single page; modals rendered as overlays
│   ├── i18n/locales/
│   │   ├── en.json
│   │   └── de.json
│   ├── nuxt.config.ts
│   └── package.json
├── backend/
│   ├── cmd/gftp/main.go        # Entry point
│   └── internal/
│       ├── api/                # HTTP handlers, middleware, routes
│       ├── auth/               # Session, CSRF, login throttle
│       ├── sso/                # SSO token encrypt/decrypt/validate
│       ├── ftp/                # goftp/ftp wrapper + operations
│       ├── sftp/               # pkg/sftp wrapper + operations
│       ├── transfer/           # Upload chunking, download, zip
│       ├── config/             # Config struct + Load()
│       ├── logging/            # slog setup, SafeLogAttrs helper
│       └── sentry/             # Sentry init + middleware
├── docker/
│   ├── Dockerfile              # Multi-stage: Go build + Nuxt build → Caddy
│   └── docker-compose.yml
├── justfile
└── settings.example.json
```

### 12.2 Container Layout (runtime)

```
/app/
  public/      # Built Nuxt SPA (static files, served by Caddy)
  gftp         # Go binary
  data/        # Volume mount — temp download/import files
/etc/caddy/Caddyfile
/entrypoint.sh
```

### 12.3 API Routes

```
POST   /api/auth/connect              # Connect + authenticate
POST   /api/auth/disconnect           # Logout
POST   /api/auth/reset-password
POST   /api/auth/forgot-password

GET    /api/files?path=               # listDirectory
POST   /api/files/directory           # makeDirectory
DELETE /api/files                     # Delete one or many items
PATCH  /api/files/rename              # rename / move
PATCH  /api/files/copy                # copy
PATCH  /api/files/permissions         # CHMOD

GET    /api/files/download            # Single file download
POST   /api/files/download-zip        # Multi-file ZIP download
GET    /api/files/size                # getRemoteFileSize

POST   /api/files/upload              # Single-shot upload
POST   /api/files/upload/reserve      # Reserve chunked upload slot
POST   /api/files/upload/chunk        # Send a chunk
POST   /api/files/upload/commit       # Finalise + transfer to remote

POST   /api/files/extract             # Extract archive in place
POST   /api/files/zip                 # Create ZIP on remote

GET    /api/system/vars               # Server capabilities + version
POST   /api/system/settings           # Persist settings.json changes
```

SSO entry point: `GET /?sso=<token>` (handled by the Go server before the SPA is served).

All API responses: `{ "success": true|false, "data": …, "errors": [] }`. Standard HTTP status codes.

### 12.4 Frontend Architecture

| Concern | Choice |
|---------|--------|
| Meta-framework | Nuxt 3, SPA mode (`ssr: false`, `nuxt generate`) |
| UI components | Nuxt UI v3 |
| Styling | Tailwind CSS v4 (bundled with Nuxt UI v3) |
| State | Pinia (`@pinia/nuxt`) |
| i18n | `@nuxtjs/i18n` v9 |
| Editor | Monaco Editor via `monaco-editor` + Nuxt plugin |
| Error tracking | `@sentry/nuxtjs` |
| Testing | Vitest + `@nuxt/test-utils` |
| Type safety | TypeScript throughout |

**Pinia stores:**

| Store | Responsibility |
|-------|---------------|
| `useAuthStore` | Connection state, session, login throttle |
| `useFilesStore` | Directory listing, selection, clipboard, nav history |
| `useTransferStore` | Upload/download queue, progress, retry state |
| `useEditorStore` | Open tabs, dirty state, auto-save |
| `useSettingsStore` | settings.json values, UI preferences |

### 12.5 Single-Modal Rule

Only one modal may be open at a time. A `useModalStore` (wrapping Nuxt UI's `useOverlay()`) enforces this: opening a new modal closes the current one first. Only one backdrop exists in the DOM at any time.

### 12.6 Auth & SSO Flow

**Standard login:**
```
Browser  →  POST /api/auth/connect  { type, host, port, username, password, … }
         ←  200 { success: true, data: { capabilities: { chmod: true }, initialDirectory: "/" } }
             Set-Cookie: gftp_session=…; HttpOnly; SameSite=Lax
```

**SSO login:**
```
External system  →  crafts encrypted token with shared GFTP_SSO_SECRET
User's browser   →  GET https://gftp.example.com/?sso=<encrypted-token>
GoblinFTP server →  decrypts token, validates exp, marks token as used,
                    creates session, sets cookie
                 →  302 Redirect  /  (clean URL, token gone from history)
Nuxt SPA loads   →  detects pre-auth session, calls /api/auth/connect
                    auto-connects with server-held credentials
```

---

## 13. Design Decisions (Resolved)

| # | Question | Decision |
|---|----------|----------|
| OQ-2 | Nuxt SPA served from same container (Caddy) or CDN? | **Same container** — Caddy serves both the Go API and the static SPA |
| OQ-3 | Does `/app/data` need persistence between restarts? | **No** — fully ephemeral; container is stateless, no volume mount required |
| OQ-4 | SSO one-time-use: in-memory set or shared store? | **In-memory** — simple single-instance enforcement; Redis not required |
