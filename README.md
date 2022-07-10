# 🔗 daunrōdo - self-hosted `yt-dlp` http server
 > Daunrōdā, ダウンローダー - means <i>downloader</i> in Japanese

Simple HTTP server that invokes [go-ytdlp](https://github.com/lrstanley/go-ytdlp). Has autoupdates, queue, progress, cache, failover capabilities. No dependencies.

Self-hosted web daemon solution tool to download original media files by extracting their direct URLs from various internet resources and social networks using plugable crawlers via API calls. More features and resources are coming by as I add them. Project started as a personal tool mainly, decided to make it public and is in early alpha stage.

Stop screenshoting, just ```daunrodo``` it!

# 🤩 Features
- [ ] 🎨 **Media** supports albums, images, videos, audios, playlists
- [ ] 🔄 **Autoupdates** `yt-dlp`, `ffmpeg` binaries installation and updates are managed automatically
- [x] 🚦 **Queue**. Requested downloads are enqueued, repeated are cached, progress (%, ETA) is tracked
- [ ] 🔀 **Failover** Failed downloads are tried again with native downloader or other tools like `gallery-dl`
- [ ] 💪 **Persistent**. Cache is a simple `.json` with files stored locally


# 📦 Installation
Best used with Docker and Traefik
```console
git clone https://github.com/wmw64/daunrodo.git && cd daunrodo
sudo docker-compose up -d
sudo docker-compose logs -f
```

# 🔬 Basic usage 
Just add your link to the daunrodo as a path. Example: ```instagram.com/p/CfwlfpcL-li/``` -> ```daunrodo.yourdomain.org/instagram.com/p/CfwlfpcL-li/```


# Motivation?
`yt-dlp` uses terminal UI so using on phones is hard. But iOS Shortcuts can invoke custom user scripts that supports HTTP requests. Just share social media post from your phone, tap daunrodo shortcut and you get back original `.mp4` video file that can be saved into gallery. No more screen recordings 🖤
I also use it as an internal microservice for my other projects to make API calls, like `blossom`, telegram bots, etc to simplify downloads logic.

# 🤝 Contributing
Contributions, issues and feature requests are welcome! 👍 <br>
Feel free to check [open issues](https://github.com/rekoda-project/rekoda/issues).

## 🌟 Show your support 
Give a ⭐️ if this project helped you!

# 📝 ToDo
- [x] Instagram crawler
- [ ] Download multiple files in one request by packing it in ZIP file
- [ ] Album image hosting downloader (cyberdrop.me, gofile.io, etc)
- [ ] CLI tool to download media from terminal

# 🧠 What I Learned
- Uncle Bob's clean architecture
- Dependency injection
- Swagger

# 📑 License 
(c) 2024 Ivan Smyshlyaev. [MIT License](https://tldrlegal.com/license/mit-license)
