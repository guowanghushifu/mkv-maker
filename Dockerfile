FROM node:24-trixie-slim AS web-build
WORKDIR /src/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.26-trixie AS go-build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM debian:trixie-slim AS runtime
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    mediainfo \
    wget \
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
    LISTEN_ADDR=:8080

COPY --from=go-build /out/server /app/server
COPY --from=web-build /src/web/dist /app/web/dist

RUN mkdir -p /app/data /bd_input /remux

EXPOSE 8080

CMD ["/app/server"]
