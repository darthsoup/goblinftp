# GoblinFTP

A self-hosted, web-based FTP/SFTP client. Deploy as a Docker container, manage remote files via browser.

Clean rewrite of [Monsta FTP](https://www.monstaftp.com/) v2.14.x with full feature parity and no licence gating.

## Stack

- **Backend:** Go + Echo
- **Frontend:** Nuxt 3 (SPA) · Nuxt UI v3 · Tailwind CSS v4
- **Container:** Docker (Caddy + Go binary)

## Quick start

```bash
docker run -p 8080:80 goblintools/gftp
```

Open http://localhost:8080

## Development

Requires: Go 1.26+, Node 20+, Docker, [just](https://just.systems), [overmind](https://github.com/DarthSim/overmind)

```bash
just dev        # start frontend + backend
just test       # run all tests
just build      # build everything
just docker-build  # build Docker image
```

See `settings.example.json` for runtime configuration options.
