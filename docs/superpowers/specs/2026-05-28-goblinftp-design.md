# GoblinFTP — Design Document

**Date:** 2026-05-28  
**Status:** Approved  
**PRD:** `PRD-GoblinFTP.md` (v0.4, updated)  
**Repo:** `darthsoup/goblinftp`

---

## 1. What We Are Building

GoblinFTP (GFTP) is a self-hosted, web-based FTP/SFTP client. Users deploy it as a Docker container and manage remote files via browser. It is a clean rewrite of Monsta FTP v2.14.x with full feature parity and no licence gating.

**Stack:**
- **Backend:** Go + Echo framework
- **Frontend:** Nuxt 3 SPA (SSR off) · Nuxt UI v3 · Tailwind CSS v4
- **Deployment:** Single Docker container (Caddy serves static SPA + reverse-proxies `/api/*` to Go binary)

---

## 2. Key Decisions Made

| # | Decision |
|---|----------|
| OQ-1 | HTTP framework: **Echo** |
| OQ-2 | Nuxt SPA in **same container** served by Caddy |
| OQ-3 | `/app/data` is **ephemeral** — no volume required |
| OQ-4 | SSO one-time-use: **in-memory set** (single-instance) |
| OQ-5 | Multi-select: **click / Shift-click / Ctrl-click** only (no rubber-band drag) |
| OQ-6 | File size: **`Content-Length` header** on download; no standalone size endpoint |
| OQ-7 | Secrets: **auto-generated** on startup with `warn` log if not set via env |
| OQ-8 | Line separator config: **dropped** — binary transfer + CodeMirror 6 handles it |
| OQ-9 | Chunk size: **manual** via `GFTP_CHUNK_SIZE` (default 5 MB); no auto-detection |
| OQ-10 | Editor: **CodeMirror 6** (not Monaco) — lightweight, modular, sufficient feature set |
| OQ-11 | Disabled login form: **HTTP 404** if no redirect configured, or **302** to `GFTP_LOGIN_DISABLED_REDIRECT` |
| OQ-12 | `reset-password` / `forgot-password` routes: **removed** (no GoblinFTP-owned credentials) |
| OQ-13 | Archive library: **`github.com/mholt/archives`** (v2, supersedes `archiver`) |

---

## 3. Architecture

### 3.1 Container Layout (runtime)

```
/app/
  public/      # Built Nuxt SPA (static, served by Caddy)
  gftp         # Go binary
  data/        # Ephemeral temp files (upload/download staging)
/etc/caddy/Caddyfile
/entrypoint.sh
```

### 3.2 Backend Package Layout

```
backend/
  cmd/gftp/main.go        # Entry point: init config, wire Echo, start server
  internal/
    api/                  # Echo handlers, middleware, route registration
    auth/                 # Session (in-memory), CSRF, login throttle
    sso/                  # AES-256-GCM token encrypt/decrypt/validate, used-token set
    ftp/                  # goftp/ftp wrapper + all file operations
    sftp/                 # pkg/sftp wrapper + all file operations
    transfer/             # Chunked upload assembly, download streaming, zip creation
    config/               # Config struct + Load() with auto-generated secrets
    logging/              # slog setup, SafeLogAttrs() credential scrubber
    sentry/               # Sentry init + Echo middleware
    errors/               # Typed error codes, FTP/SFTP error mapping
```

### 3.3 Frontend Component Layout

```
frontend/
  layouts/
    default.vue           # Wires AppHeader + main content + AppFooter
  pages/
    index.vue             # Single page
  components/
    Layout/
      AppHeader.vue       # Top bar: logo, connection status, user actions
      AppFooter.vue       # Status bar: current path, selection count, transfer indicator
      AppSidebar.vue      # Optional nav / bookmarks panel
      Breadcrumb.vue      # Path navigation bar
    FileBrowser/
      FileTable.vue       # Directory listing table
      FileRow.vue         # Single file/folder row
      ContextMenu.vue     # Right-click menu
    Transfer/
      UploadDropZone.vue  # Drag-and-drop overlay
      TransferQueue.vue   # Upload/download progress list
      TransferItem.vue    # Single transfer row with progress + cancel
    Editor/
      EditorPane.vue      # CodeMirror 6 wrapper (lazy async component)
      EditorTabBar.vue    # Open file tabs with dirty indicator
    Modals/
      RenameModal.vue
      DeleteModal.vue
      ChmodModal.vue
      PropertiesModal.vue
      NewFileModal.vue
      NewFolderModal.vue
    Auth/
      LoginForm.vue
  stores/
    auth.ts               # Connection state, session, login throttle
    files.ts              # Directory listing, selection, clipboard, nav history
    transfer.ts           # Upload/download queue, progress, retry state
    editor.ts             # Open tabs, dirty flags, auto-save
    settings.ts           # settings.json values, UI preferences
    modal.ts              # Single-modal enforcement via useOverlay()
  i18n/locales/
    en.json
    de.json
```

---

## 4. API Contract

### 4.1 Response Envelope

All endpoints return:
```json
{ "success": true, "data": { … } }
{ "success": false, "errors": [{ "code": "ERR_KEY", "message": "…" }] }
```

### 4.2 Routes

```
POST   /api/auth/connect              # Connect + authenticate → sets gftp_session cookie
POST   /api/auth/disconnect           # Logout

GET    /api/files?path=               # List directory
POST   /api/files/directory           # Create directory
DELETE /api/files                     # Delete one or many items
PATCH  /api/files/rename              # Rename / move
PATCH  /api/files/copy                # Copy
PATCH  /api/files/permissions         # CHMOD

GET    /api/files/download            # Stream file (Content-Length from Stat(); HMAC token required)
POST   /api/files/download-zip        # Assemble + stream ZIP of selected items

POST   /api/files/upload              # Single-shot upload (small files)
POST   /api/files/upload/reserve      # Reserve chunked upload slot → { uploadId }
POST   /api/files/upload/chunk        # Send a chunk → { chunkId }
POST   /api/files/upload/commit       # Assemble + transfer to remote

POST   /api/files/extract             # Extract archive in-place on server
POST   /api/files/zip                 # Create ZIP from selected items on server

GET    /api/system/vars               # Server capabilities + version
POST   /api/system/settings           # Persist settings.json changes
```

SSO entry: `GET /?sso=<token>` — handled by Go server before SPA is served.

### 4.3 Auth Flow

```
Browser  →  POST /api/auth/connect  { type, host, port, username, password, … }
         ←  200 { success: true, data: { capabilities: { chmod: bool }, initialDirectory: "/" } }
             Set-Cookie: gftp_session=…; HttpOnly; SameSite=Lax
```

Session stored in-memory on the Go server. Credentials never re-sent after connect.

### 4.4 Chunked Upload Flow

```
POST /api/files/upload/reserve  →  { uploadId }
POST /api/files/upload/chunk    →  { chunkId }   (repeated, 5 MB default per chunk)
POST /api/files/upload/commit   →  streams assembled file to FTP/SFTP
```

Retry: up to 5 attempts with exponential backoff per chunk on transient errors.

### 4.5 Download Flow

```
GET /api/files/download?path=…&token=…
```

Token: HMAC-SHA256 signed with `GFTP_DOWNLOAD_TOKEN_SECRET`, contains path + expiry. Backend validates, streams file with `Content-Length` from `Stat()`.

---

## 5. Security

| Control | Implementation |
|---------|---------------|
| CSRF | Token on all state-changing API requests |
| Session fixation | Session ID regenerated on login |
| Login throttle | Per host+user, configurable max attempts + cooldown |
| SSO tokens | AES-256-GCM encrypted, one-time use (in-memory set), 15 min TTL default |
| Download tokens | HMAC-SHA256 signed, path-scoped |
| Credential logging | `SafeLogAttrs()` strips passwords/keys before any log call |
| IP allowlist | `allowedClientAddresses` in settings.json |
| Login form disabled | 404 or configurable 302 redirect |
| CSP | Content-Security-Policy header on all HTML responses |

---

## 6. Error Handling

**Backend:**
- FTP/SFTP protocol errors mapped to typed codes in `internal/errors/` — raw errors never reach the client
- Panics caught by Echo recovery middleware, sent to Sentry, returned as generic 500
- All log calls go through `SafeLogAttrs()` — no passwords or key material in logs

**Frontend:**
- API composable maps `errors[].code` → i18n string → toast notification
- Network timeouts show retry prompt in transfer queue
- Unhandled JS exceptions captured by `@sentry/nuxtjs`

---

## 7. Testing

| Layer | Tool | Target |
|-------|------|--------|
| Go backend | `testing` + `testify` | ≥ 70% line coverage on `internal/` packages |
| FTP/SFTP clients | Interface-wrapped + mocked | No live server required in unit tests |
| Pinia stores | Vitest + `@pinia/testing` | 100% store action coverage |
| Vue components | Vitest + `@nuxt/test-utils` | Key flows: login, upload, delete, editor open/save |
| E2E | Not in scope for v1 | — |

---

## 8. Implementation Phases

| Phase | Deliverable |
|-------|-------------|
| 1 — Scaffold | Repo structure, justfile, Dockerfile skeleton, CI workflow |
| 2 — Backend core | Config, logging, session/CSRF/auth middleware, Echo router, error package |
| 3 — FTP/SFTP layer | Connect, list, download, upload (chunked), rename, delete, copy, chmod, archive |
| 4 — SSO & security | SSO token decrypt/validate, IP allowlist, download HMAC tokens |
| 5 — Frontend | Nuxt SPA: auth, file browser, transfer UI, editor (CodeMirror 6), settings |
| 6 — Polish | German i18n, Sentry integration, test coverage to ≥ 70% / 100% stores |
