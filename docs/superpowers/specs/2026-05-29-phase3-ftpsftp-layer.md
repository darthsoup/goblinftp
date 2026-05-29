# Phase 3 — FTP/SFTP Layer Design

**Date:** 2026-05-29  
**Status:** Approved  
**Scope:** Complete the Connect handler, implement all 15 file-operation routes, download tokens, chunked upload, and system vars.

---

## 1. Overview

Phase 3 turns the stub handlers registered in Phase 2 into working file operations. The central design decision is a unified `transfer.Client` interface that both FTP and SFTP adapters implement. Handlers never call protocol libraries directly — they call the interface. This keeps handlers clean, makes mocking trivial, and avoids protocol-specific conditionals scattered across 12+ handlers.

Phase 3 leaves `POST /api/system/settings` as `501 Not Implemented` — it is deferred to Phase 5 when the frontend settings page is built.

---

## 2. New Package Structure

```
backend/internal/
  transfer/
    client.go      — Client interface, FileInfo, errors
    token.go       — HMAC-SHA256 download token (generate + validate)
    upload.go      — Chunked upload state (reserve / chunk / commit)
  ftp/
    ftp.go         — FTP Client implementation (goftp/ftp)
    ftp_test.go
  sftp/
    sftp.go        — SFTP Client implementation (x/crypto/ssh + pkg/sftp)
    sftp_test.go
  api/
    connect.go     — Updated: dial + session store
    files.go       — List, mkdir, delete, rename, copy, chmod
    download.go    — Single file + multi-file ZIP
    upload.go      — Simple + chunked (reserve/chunk/commit)
    archive.go     — Extract + create ZIP
    system.go      — GET /api/system/vars
```

New Go module dependencies to add:
- `github.com/goftp/ftp` — FTP client
- `github.com/pkg/sftp` — SFTP client
- `github.com/mholt/archives` — tar.gz / tar.bz2 extraction

`golang.org/x/crypto/ssh` is already an indirect dependency — promote to direct.

**New config variables** (add to `config.Config` and `config_test.go`):

| Variable | Default | Description |
|---|---|---|
| `GFTP_DOWNLOAD_TOKEN_SECRET` | (required) | 32+ byte hex/random secret for HMAC token signing |
| `GFTP_DATA_DIR` | `/app/data` | Temp storage for chunked uploads and archive operations |

---

## 3. `transfer.Client` Interface

```go
// FileInfo holds metadata for a single remote file or directory.
type FileInfo struct {
    Name        string
    Size        int64
    ModTime     time.Time
    IsDir       bool
    Permissions uint32 // Unix permission bits (e.g. 0o755)
}

// Client is the unified interface for FTP and SFTP adapters.
// All operations use slash-separated paths relative to the server root.
// Close must be called when the session ends or on disconnect.
type Client interface {
    List(path string) ([]FileInfo, error)
    Stat(path string) (*FileInfo, error)
    MkDir(path string) error
    Delete(path string) error
    Rename(oldPath, newPath string) error
    Copy(src, dst string) error
    SetPermissions(path string, mode uint32) error
    Download(path string) (io.ReadCloser, int64, error) // reader, size, error
    Upload(path string, r io.Reader) error
    Walk(root string, fn func(path string, info FileInfo) error) error
    WorkingDir() (string, error)
    Close() error
}
```

**Copy semantics:** FTP has no native copy; the FTP adapter downloads and re-uploads. SFTP also has no POSIX copy command; the SFTP adapter does the same. This is acceptable — copy is an uncommon operation on large files.

**Permissions:** FTP uses `SITE CHMOD`; SFTP uses `Chmod()`. The adapter returns a `transfer.ErrPermissionsNotSupported` error when the server does not support it. The handler translates this to a `200 { capabilities.chmod: false }` on connect, and the route returns a user-facing error if called anyway.

---

## 4. Session Storage

The connected `transfer.Client` is stored in `Session.Data["client"]` after successful `Connect`. All file handlers retrieve it via:

```go
sess := c.Get("session").(*auth.Session)
client, ok := sess.Data["client"].(transfer.Client)
if !ok {
    return Fail(c, gftperrors.New(gftperrors.ErrUnauthorized, "no active connection"))
}
```

The `Disconnect` handler calls `client.Close()` before deleting the session.

---

## 5. Updated `Connect` Handler

After the existing Phase 2 validation (type → fields → port → IP allowlist → throttle), Phase 3 adds:

1. Dial the server using the appropriate adapter (`ftp.New(req)` or `sftp.New(req)`).
2. On auth failure: record throttle failure, return `ErrAuthFailed`.
3. On success: reset throttle, create a new session, store the client, set the session cookie.
4. Determine `initialDirectory`: call `client.WorkingDir()`; fall back to `"/"` on error.
5. Detect `chmod` capability: FTP — always true (all servers support SITE CHMOD); SFTP — attempt a no-op `Chmod()`, catch `ErrPermissionsNotSupported`.
6. Generate a CSRF token, store in session, return `ConnectData`.

**ConnectRequest** gains two new optional fields for SFTP key auth (Phase 3 supports password only; key auth is Phase 4):

```go
type ConnectRequest struct {
    Type     string `json:"type"`
    Host     string `json:"host"`
    Port     int    `json:"port"`
    Username string `json:"username"`
    Password string `json:"password"`
    Passive  bool   `json:"passive"`  // FTP only, default true
}
```

---

## 6. File Operation Handlers

All handlers follow the same pattern:
1. Extract `transfer.Client` from session.
2. Bind and validate the request body / query params.
3. Call the client method.
4. Return `OK(c, data)` or `Fail(c, err)`.

### 6.1 `GET /api/files?path=`

Returns a sorted directory listing. Response:

```json
{
  "success": true,
  "data": {
    "path": "/home/user",
    "entries": [
      { "name": "docs", "size": 0, "modTime": "2026-01-01T00:00:00Z", "isDir": true, "permissions": 493 },
      { "name": "readme.txt", "size": 1024, "modTime": "2026-01-01T00:00:00Z", "isDir": false, "permissions": 420 }
    ]
  }
}
```

Entries are sorted: directories first, then files, both groups sorted alphabetically case-insensitively.

### 6.2 `POST /api/files/directory` — `{ "path": "/new/dir" }`

### 6.3 `DELETE /api/files` — `{ "paths": ["/a", "/b"] }`

Deletes each path in order. Continues on individual errors, returns a partial-failure response if any deletions failed (HTTP 207 Multi-Status). A fully successful delete returns 200.

### 6.4 `PATCH /api/files/rename` — `{ "from": "/old", "to": "/new" }`

### 6.5 `PATCH /api/files/copy` — `{ "from": "/src", "to": "/dst" }`

### 6.6 `PATCH /api/files/permissions` — `{ "path": "/file", "permissions": 493 }`

Permissions are passed as a decimal integer (Unix mode bits).

---

## 7. Download Handlers

### 7.1 Download Token Scheme

Single-file downloads use HMAC-signed tokens to allow direct browser downloads without per-request CSRF. The token is short-lived and session-bound.

**Token format:** `base64url(sessionID ":" path ":" expiry ":" hmac)`  
**Signature:** `HMAC-SHA256(sessionID + ":" + path + ":" + expiry, GFTP_DOWNLOAD_TOKEN_SECRET)`  
**Expiry:** 5 minutes from issue

`POST /api/files/download-token` (new route added to router) — body `{ "path": "/file.txt" }` — returns `{ "token": "..." }`.

`GET /api/files/download?token=...` — validates token (signature, expiry, session still active), streams file directly with `Content-Disposition: attachment`.

### 7.2 Multi-file ZIP — `POST /api/files/download-zip`

Body: `{ "paths": ["/a", "/b/c"] }`

The handler:
1. Creates a `zip.Writer` piped directly to the response body (`Content-Type: application/zip`, `Transfer-Encoding: chunked`).
2. Walks each path (recursively for directories) using `client.Walk`.
3. Downloads each file and streams it into the ZIP — no temp file on disk.
4. On error mid-stream, the TCP connection is closed (partial ZIP is unusable; client sees an error).

---

## 8. Upload Handlers

### 8.1 Simple Upload — `POST /api/files/upload`

Multipart form: `file` field + `path` query param. Streams directly to `client.Upload()` without buffering to disk. Respects `GFTP_MAX_FILE_SIZE` via Echo's `BodyLimit` middleware (applied at the route level, not globally, to avoid affecting other routes).

### 8.2 Chunked Upload

Used for large files where the frontend splits the file client-side.

**Reserve — `POST /api/files/upload/reserve`**  
Body: `{ "filename": "video.mp4", "totalSize": 104857600, "chunkCount": 20 }`  
Creates a temp directory under `GFTP_DATA_DIR/{uploadID}/`. Returns `{ "uploadID": "uuid" }`.  
Upload ID is stored in `Session.Data["uploads"]` (map of uploadID → `uploadMeta`).

**Chunk — `POST /api/files/upload/chunk?uploadID=X&index=N`**  
Raw body = chunk bytes. Writes to `{dataDir}/{uploadID}/{N:04d}`. Returns `{ "received": N }`.

**Commit — `POST /api/files/upload/commit`**  
Body: `{ "uploadID": "uuid", "path": "/remote/video.mp4" }`  
Reads chunks in order, streams assembled content to `client.Upload()` via a `io.MultiReader`. Cleans up temp dir on success or failure.

---

## 9. Archive Handlers

### 9.1 Extract — `POST /api/files/extract`

Body: `{ "path": "/archive.zip", "destination": "/extract/to/" }`

1. Download the archive to a temp file in `GFTP_DATA_DIR`.
2. Extract using `archive/zip` (ZIP) or `mholt/archives` (tar.gz, tar.bz2). Supported formats detected by file extension.
3. Upload extracted files to the destination path using `client.Upload`.
4. Clean up temp file.

### 9.2 Create ZIP — `POST /api/files/zip`

Body: `{ "paths": ["/a", "/b"], "destination": "/archive.zip" }`

1. Walk each path, download each file into a local `zip.Writer` writing to a temp file.
2. Upload the temp ZIP to `destination` via `client.Upload`.
3. Clean up temp file.

---

## 10. System Vars — `GET /api/system/vars`

Returns configuration values the frontend needs. Does not require authentication (called on page load before connect, to configure the login form).

```json
{
  "success": true,
  "data": {
    "chunkUploadSize": 5242880,
    "maxFileUpload": 2147483648,
    "maxConcurrentUploads": 10,
    "sshAgentAuthEnabled": false,
    "sshKeyAuthEnabled": false,
    "version": "0.1.0",
    "applicationSettings": {
      "language": "en",
      "ui": { "pageTitle": "GoblinFTP", "showDotFiles": false, "showNavigationHistory": true, "helpUrl": null },
      "editor": { "openOnCreate": false, "allowedExtensions": ["txt", "php", ...], "disabled": false, "viewOnly": false },
      "connection": { "allowedTypes": ["ftp", "sftp"], "disableChmod": false, "requestTimeoutSeconds": 30 },
      "access": { "allowedClientAddresses": [], "deniedMessage": null, "postLogoutUrl": null }
    }
  }
}
```

`/api/system/vars` skips the `requireSession` middleware. It is listed before the CSRF middleware applies (GET is always exempt from CSRF anyway). The `applicationSettings.access.allowedClientAddresses` is **omitted** from the response (security — the client should not know the IP whitelist).

**Route change:** Move `GET /api/system/vars` out of the `requireSession` group in `router.go`.

---

## 11. Error Handling

New error codes to add to `internal/errors/errors.go`:

| Code | HTTP | Meaning |
|---|---|---|
| `ErrAuthFailed` | 401 | FTP/SFTP login credentials rejected |
| `ErrConnectionFailed` | 502 | Could not reach the remote server |
| `ErrConnectionTimeout` | 504 | Remote server did not respond in time |
| `ErrPermissionsNotSupported` | 422 | Server does not support CHMOD |
| `ErrUploadNotFound` | 404 | Upload ID does not exist or expired |
| `ErrInvalidToken` | 401 | Download token missing, invalid, or expired |
| `ErrArchiveFormat` | 422 | Unsupported or corrupt archive format |

---

## 12. Testing Strategy

Each new package gets a `_test.go` with unit tests. The FTP and SFTP adapters use interface-based mocking — a `MockClient` struct in `transfer/mock_test.go` allows handler tests to exercise all code paths without a real server.

Test targets:
- `transfer/token.go` — generate + validate (expiry, tampering, wrong session)
- `transfer/upload.go` — reserve / chunk / commit lifecycle, cleanup on error
- `ftp/ftp.go` — integration tests skipped unless `GFTP_TEST_FTP_HOST` is set
- `sftp/sftp.go` — integration tests skipped unless `GFTP_TEST_SFTP_HOST` is set
- `api/files.go`, `api/download.go`, `api/upload.go`, `api/archive.go`, `api/system.go` — unit tests via `MockClient`
- `api/connect.go` — existing tests updated for full connect flow; mock adapter injected

**MockClient** is a `transfer.Client` implementation that records calls and returns configurable responses. It lives in a `testutil` sub-package under `transfer/`.

---

## 13. Implementation Tasks

| # | Task | Package(s) |
|---|---|---|
| T1 | Add new error codes | `internal/errors` |
| T2 | Define `transfer.Client` interface, `FileInfo`, `MockClient` | `internal/transfer` |
| T3 | Download token (generate + validate) | `internal/transfer` |
| T4 | Chunked upload state management | `internal/transfer` |
| T5 | FTP adapter | `internal/ftp` |
| T6 | SFTP adapter | `internal/sftp` |
| T7 | Complete `Connect` handler | `internal/api` |
| T8 | File operation handlers (list, mkdir, delete, rename, copy, chmod) | `internal/api` |
| T9 | Download handlers (single + ZIP) | `internal/api` |
| T10 | Upload handlers (simple + chunked) | `internal/api` |
| T11 | Archive handlers (extract + create ZIP) | `internal/api` |
| T12 | System vars handler | `internal/api` |
| T13 | Add new dependencies (`go get`) + update `go.mod` | |
| T14 | Wire new handlers into `Handler` struct + `router.go`; add `POST /api/files/download-token`; move `GET /api/system/vars` out of `requireSession`; add new config vars | `internal/api`, `internal/config` |
