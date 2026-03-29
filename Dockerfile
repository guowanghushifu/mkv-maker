FROM node:20-bookworm-slim AS web-builder

WORKDIR /src/web
COPY web/package*.json ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.21-bookworm AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM debian:bookworm-slim AS makemkv-builder

ARG MAKEMKV_VERSION=1.18.1
ENV MAKEMKV_VERSION=${MAKEMKV_VERSION}
ENV MAKEMKV_PREFIX=/opt/makemkv

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      build-essential \
      pkg-config \
      libc6-dev \
      libssl-dev \
      zlib1g-dev \
      libexpat1-dev \
      libavcodec-dev \
      libgl1-mesa-dev && \
    rm -rf /var/lib/apt/lists/*

COPY scripts/makemkv-build.sh /usr/local/bin/makemkv-build.sh
RUN chmod +x /usr/local/bin/makemkv-build.sh && /usr/local/bin/makemkv-build.sh

FROM debian:bookworm-slim

ARG MAKEMKV_VERSION=1.18.1
ENV MAKEMKV_VERSION=${MAKEMKV_VERSION}
ENV APP_DATA_DIR=/app/data
ENV BD_INPUT_DIR=/bd_input
ENV REMUX_OUTPUT_DIR=/remux
ENV LISTEN_ADDR=:8080
ENV MAKEMKV_CONFIG_DIR=/app/data/makemkv
ENV MAKEMKV_KEY=BETA
ENV PATH=/opt/makemkv/bin:$PATH
ENV LD_LIBRARY_PATH=/opt/makemkv/lib

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      ffmpeg \
      mediainfo \
      mkvtoolnix \
      libexpat1 \
      libssl3 \
      zlib1g \
      libavcodec59 \
      libstdc++6 \
      libgcc-s1 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=go-builder /out/server /app/server
COPY --from=web-builder /src/web/dist /app/web/dist
COPY --from=makemkv-builder /opt/makemkv /opt/makemkv
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY scripts/makemkv-set-key.sh /usr/local/bin/makemkv-set-key.sh
COPY scripts/makemkv-update-beta-key.sh /usr/local/bin/makemkv-update-beta-key.sh
COPY docker/makemkv/settings.conf /etc/makemkv/settings.conf

RUN chmod +x \
      /app/server \
      /usr/local/bin/docker-entrypoint.sh \
      /usr/local/bin/makemkv-set-key.sh \
      /usr/local/bin/makemkv-update-beta-key.sh && \
    mkdir -p /app/data/makemkv/data /bd_input /remux

EXPOSE 8080
VOLUME ["/app/data", "/bd_input", "/remux"]

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["/app/server"]
