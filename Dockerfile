FROM node:24-trixie-slim AS web-build
ARG BUILD_TIME
ENV VITE_BUILD_TIME=${BUILD_TIME}
WORKDIR /src/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

FROM --platform=$BUILDPLATFORM debian:13 AS makemkv-build
ARG TARGETPLATFORM
ARG MAKEMKV_VERSION=1.18.3
ARG MAKEMKV_OSS_URL=https://www.makemkv.com/download/makemkv-oss-${MAKEMKV_VERSION}.tar.gz
ARG MAKEMKV_BIN_URL=https://www.makemkv.com/download/makemkv-bin-${MAKEMKV_VERSION}.tar.gz
COPY --from=xx / /
COPY docker/makemkv-build /build
RUN /build/build.sh "${MAKEMKV_OSS_URL}" "${MAKEMKV_BIN_URL}"
RUN xx-verify \
    /opt/makemkv/bin/makemkvcon \
    /opt/makemkv/lib/libmakemkv.so.1 \
    /opt/makemkv/lib/libdriveio.so.0 \
    /opt/makemkv/lib/libmmbd.so.0

FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS go-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/server ./cmd/server

FROM debian:trixie-slim AS runtime
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    cron \
    sed \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /etc/apt/keyrings && \
    wget -O /etc/apt/keyrings/gpg-pub-moritzbunkus.gpg https://mkvtoolnix.download/gpg-pub-moritzbunkus.gpg && \
    printf '%s\n' \
      'deb [signed-by=/etc/apt/keyrings/gpg-pub-moritzbunkus.gpg] https://mkvtoolnix.download/debian/ trixie main' \
      > /etc/apt/sources.list.d/mkvtoolnix.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends mkvtoolnix && \
    rm -rf /var/lib/apt/lists/*

ENV APP_DATA_DIR=/app/data \
    BD_INPUT_DIR=/bd_input \
    REMUX_OUTPUT_DIR=/remux \
    REMUX_TMP_DIR=/remux_tmp \
    LISTEN_ADDR=:8080 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    LANGUAGE=C.UTF-8

COPY --from=go-build /out/server /app/server
COPY --from=web-build /src/web/dist /app/web/dist
COPY --from=makemkv-build /opt/makemkv /opt/makemkv
COPY docker/makemkv/bin/makemkv-update-beta-key /opt/makemkv/bin/makemkv-update-beta-key
COPY docker/makemkv/bin/makemkv-set-key /opt/makemkv/bin/makemkv-set-key
COPY docker/makemkv/defaults/settings.conf /defaults/settings.conf
COPY docker/makemkv/defaults/nocore.mmcp.xml /defaults/nocore.mmcp.xml
COPY docker/cron.d/makemkv-beta-key /etc/cron.d/makemkv-beta-key
COPY docker/entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod 755 \
      /usr/local/bin/docker-entrypoint.sh \
      /opt/makemkv/bin/makemkv-update-beta-key \
      /opt/makemkv/bin/makemkv-set-key && \
    chmod 644 /etc/cron.d/makemkv-beta-key && \
    mkdir -p /app/data /bd_input /remux /remux_tmp /config /config/data /defaults && \
    touch /var/log/makemkv-beta-key.log

VOLUME ["/config"]

EXPOSE 8080

CMD ["/usr/local/bin/docker-entrypoint.sh"]
