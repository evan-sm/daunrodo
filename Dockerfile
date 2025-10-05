FROM --platform=$BUILDPLATFORM golang:bookworm AS build

ARG TARGETOS=linux
ARG TARGETARCH=arm64

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ENV CGO_ENABLED=0

# build for the TARGET (not the builder)
# RUN --mount=type=cache,target=/root/.cache/go-build \
#     GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-s -w" -o app ./...
RUN --mount=type=cache,target=/root/.cache/go-build \
    echo ">> building for TARGETOS=$TARGETOS TARGETARCH=$TARGETARCH (BUILDPLATFORM=$BUILDPLATFORM)" && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o app
    
# FROM golang:bookworm AS build

# RUN apk add --update --no-cache tzdata

# WORKDIR /app
# COPY . .
# RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o app

FROM debian:bookworm-slim

ENV TZ=Europe/Moscow

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates ffmpeg \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -u 1000 -m -d /home/app -s /usr/sbin/nologin app && \
    mkdir -p /app/data/downloads /app/data/cache /app/.cache && \
    chown -R app:app /app /home/app
ENV XDG_CACHE_HOME=/app/.cache


#COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
#COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
#COPY --from=build /etc/passwd /etc/passwd
#COPY --from=build /etc/group /etc/group

WORKDIR /app
COPY --from=build /app/app /app/app
# COPY --from=build /app/entrypoint.sh /app/entrypoint.sh

USER 1000:1000

ENTRYPOINT ["/app/app"]
