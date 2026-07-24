# Frontend first: it changes far less often than the Go code, so keeping it in
# its own stage means a Go edit does not reinstall node_modules.
FROM node:22-alpine AS web

WORKDIR /src/web
# Copy the manifests alone before the sources, so `npm ci` is only re-run when
# dependencies actually change.
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --fund=false

COPY web/ ./
COPY internal/webui/dist/.gitkeep /src/internal/webui/dist/.gitkeep
RUN npm run build


FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Vite writes here; take it from the web stage rather than rebuilding.
COPY --from=web /src/internal/webui/dist ./internal/webui/dist

# CGO_ENABLED=0 is the whole deploy story: a static binary that runs on any
# base image, including scratch. The daemon only depends on a pure-Go SQLite
# driver, so this holds without a C toolchain in the image.
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/rcon-ui ./cmd/rcon-ui


# scratch rather than alpine: the binary is static and the attack surface of a
# container holding RCON passwords should be exactly one file. No shell means no
# shell to exploit.
FROM scratch

# CA certificates are not needed for RCON itself, but are required if the daemon
# is ever put behind an egress proxy or gains an outbound HTTPS feature.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/rcon-ui /rcon-ui

# 65532 is the conventional "nonroot" uid. Declared numerically because there is
# no /etc/passwd in scratch for a name to resolve against.
USER 65532:65532

# The data directory must be a volume: it holds the SQLite database and, unless
# RCON_UI_KEY is supplied, the key that unseals every stored password.
ENV RCON_UI_DATA_DIR=/data
VOLUME ["/data"]

# Binds to all interfaces because a container's loopback is unreachable from
# outside it. That makes RCON_UI_TOKEN mandatory in practice -- see the Helm
# chart, which refuses to render without one.
ENV RCON_UI_ADDR=0.0.0.0:8477
EXPOSE 8477

ENTRYPOINT ["/rcon-ui"]
CMD ["serve"]
