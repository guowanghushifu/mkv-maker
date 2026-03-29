FROM node:20-bookworm-slim AS web-build
WORKDIR /src/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.21-bookworm AS go-build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM debian:bookworm-slim AS runtime
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    mediainfo \
    mkvtoolnix \
    && rm -rf /var/lib/apt/lists/*

ENV APP_DATA_DIR=/app/data \
    BD_INPUT_DIR=/bd_input \
    REMUX_OUTPUT_DIR=/remux \
    LISTEN_ADDR=:8080

COPY --from=go-build /out/server /app/server
COPY --from=web-build /src/web/dist /app/web/dist

RUN mkdir -p /app/data /bd_input /remux

EXPOSE 8080

CMD ["/app/server"]
