# The image Railway builds and runs (railway.toml): the PWA compiled and
# embedded in the Go binary, on an image with nothing else in it.

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# Writes the PWA into /src/api/cmd/server/dist, where the binary embeds it.
RUN npm run build

FROM golang:1.26-alpine AS api
WORKDIR /src/api
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
COPY --from=web /src/api/cmd/server/dist ./cmd/server/dist
# Railway passes the commit being deployed; GET /api/v1/health reports it.
ARG RAILWAY_GIT_COMMIT_SHA=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${RAILWAY_GIT_COMMIT_SHA}" -o /quitsie ./cmd/server

# No shell and no root: only the binary and the CA certificates it needs to
# reach PostgreSQL, Resend and Google over TLS.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /quitsie /quitsie
ENTRYPOINT ["/quitsie"]
