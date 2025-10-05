# 🔗 daunrōdo - self-hosted `yt-dlp` http server
 > Daunrōdā, ダウンローダー - means <i>downloader</i> in Japanese

Simple self-hosted HTTP API around [yt-dlp](https://github.com/lrstanley/go-ytdlp) to download media from links. Has autoupdates, queue, progress, cache, failover capabilities. No dependencies.


Stop screenshoting, just ```daunrodo``` it!

# 🤩 Features
- [x] 🎨 **Media** supports videos, audios, playlists
- [x] 🔄 **Autoinstall** `yt-dlp`, `ffmpeg` binaries installation are managed automatically
- [x] 🚦 **Queue**. Requested downloads are enqueued, cached, cleaned (TTL) and progress (%, ETA) is tracked

# ✅ TO DO
- [ ] 🔀 **Failover** Failed downloads are tried again with native downloader or other tools like `gallery-dl`
- [ ] 💪 **Persistent**. Jobs are stored in a simple `.json` file for statefulness
- [ ] 📂 **Packaging**. Multi-file `.zip` archive packaging


# 📦 Quick Start
Best used with Docker and Traefik
```console
git clone https://github.com/evan-sm/daunrodo.git && cd daunrodo
sudo docker-compose up -d
sudo docker-compose logs -f
```

# 🔬 Basic usage 
Enqueue:
```shell
curl -X POST localhost:8080/v1/jobs/enqueue -d '{"url":"youtube.com/watch?v=dQw4w9WgXcQ","preset":"mp4"}'
```
Poll job (is status finished?):
```shell
curl -s localhost:8080/v1/jobs/<job-uuid>
```
List jobs:
```shell
curl -s localhost:8080/v1/jobs/
```
Download file:
```shell
wget localhost:8080/v1/files/<publication-uuid>
```


# 💪🏻 Motivation?
`yt-dlp` uses terminal UI so using on phones is hard. But iOS Shortcuts can invoke custom user scripts that supports HTTP requests. Just share social media post from your phone, tap daunrodo shortcut and you get back original `.mp4` video file that can be saved into gallery. No more screen recordings 🖤
I also use it as an internal microservice for my other projects to make API calls, like `blossom`, telegram bots, etc to simplify downloads logic.

# 🧠 What I Learned
- Stdlib `net/http` usage with updated `ServeMux()` patterns in Go 1.22
- Composable HTTP middlewares without frameworks (panics, request IDs, logging)
- Graceful shutdown lifecylce with context timeouts
- UUIDv5 for idempotency and cache reuse
- Worker-pool patterns
- Proper structured logging using `log/slog`
- In-memory storage with TTL and periodic cleanup including deletion of expired files
- Simple yet flexible unit testing using real filesystems files


# 📑 License 
(c) 2025 Ivan Smyshlyaev. [MIT License](https://tldrlegal.com/license/mit-license)
