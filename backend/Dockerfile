# syntax=docker/dockerfile:1.7
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum* ./backend/
WORKDIR /src/backend
RUN --mount=type=secret,id=corp_ca,required=false \
    if [ -f /run/secrets/corp_ca ]; then cp /run/secrets/corp_ca /usr/local/share/ca-certificates/restoreguard-local.crt && update-ca-certificates; fi && \
    go mod download && \
    if [ -f /usr/local/share/ca-certificates/restoreguard-local.crt ]; then rm /usr/local/share/ca-certificates/restoreguard-local.crt && update-ca-certificates; fi
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/restoreguard ./cmd/server

FROM docker:29-cli AS dockercli
FROM alpine:3.22
RUN addgroup -S restoreguard && adduser -S -G restoreguard restoreguard
WORKDIR /app
COPY --from=build /out/restoreguard /usr/local/bin/restoreguard
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=dockercli /usr/local/bin/docker /usr/local/bin/docker
COPY migrations /app/migrations
RUN mkdir -p /var/lib/restoreguard/artifacts && chown -R restoreguard:restoreguard /var/lib/restoreguard /app
USER restoreguard
EXPOSE 8080
ENTRYPOINT ["restoreguard"]
