# Agent Guide for daunrodo

This repo is a Go 1.25+ self-hosted HTTP API around `yt-dlp` for media downloads.

## Quick Commands
- Build: `go build -o app` or `go build -ldflags "-s -w" -o app`
- Run: `./app` or `go run main.go`
- Test: `go test -v -cover ./...`
- Lint: `golangci-lint run` (or `task lint`)
- Docker: `docker build -t daunrodo .`, `docker compose up -d`

More details live in `BUILD.md`.

## Layout
- `main.go`: application entry point and wiring.
- `internal/`: app code (config, service, storage, downloader, http).
  - `depmanager/`: binary dependency management (yt-dlp, ffmpeg, gallery-dl) with SHA256 verification.
  - `proxymgr/`: proxy rotation with health checking and failure tracking.
  - `observability/`: Prometheus metrics for jobs, downloads, proxies, HTTP.
- `pkg/`: shared utilities (logging, http helpers, etc.).
- `data/`: runtime download/cache data (do not commit generated files).

## Conventions
- Environment variables are prefixed with `DAUNRODO_`; keep new config in `internal/config`.
- No third-party dependencies preferred (exception: prometheus client for metrics).
- Logging uses `log/slog`; prefer structured fields.
- New downloaders should implement `Downloader` in `internal/downloader` and add a const in `internal/consts`.
- Tests should avoid network and reuse `internal/downloader/testdata` when parsing tool output.
- Run `gofmt` on Go files and keep exported comments up to date.
- Avoid using `helper` or `utils` packages for small helper methods. Instead use and place them in `./pkg/` folder. If needed create or use existing folder and package, consolidate funcs together, always write table unit-tests with standard library.
- Errors variables and constans are always placed in `./internal/errs/errs.go` and `./internal/consts/consts.go` files. Always comment them or their blocks.
- Check for linter. We should have 0 issues, if got any rewrite code to fix it.
- Never delete or rewrite existing `_test.go` files. Only append new tests or adjust imports.
- If a test fails, fix code first; only adjust the test when you can explain the behavior change.
- In error messages avoid using words like "failed to", "error", "unable to".

## Key Packages
- `depmanager.Manager`: manages binary downloads with `MustInstallAll()`, `GetInstalledPath()`, `StartUpdateChecker()`.
- `proxymgr.Manager`: proxy rotation with `GetRandomProxy()`, `MarkFailed()`, `MarkSuccess()`, `StartHealthChecker()`.
- `downloader.YTdlp`: yt-dlp downloader using `exec.Command` directly.
- `downloader.GalleryDL`: gallery-dl downloader for TikTok image slideshows.

## When Changing Behavior
- Update `README.md` and/or `BUILD.md` if you add new flags, env vars, or workflow steps.
