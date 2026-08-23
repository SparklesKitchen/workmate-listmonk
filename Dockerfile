FROM node:24-alpine AS frontend

WORKDIR /src
COPY static ./static
COPY frontend/package.json frontend/yarn.lock ./frontend/
WORKDIR /src/frontend
RUN corepack enable && yarn install --frozen-lockfile
COPY frontend/ ./
WORKDIR /src/frontend/email-builder
RUN corepack enable && yarn install --frozen-lockfile && yarn build
RUN mkdir -p /src/frontend/public/static/email-builder && cp dist/* /src/frontend/public/static/email-builder/
WORKDIR /src/frontend
# The owner console is mounted below WorkMate SaaS Admin. Compile both the
# router/assets and API client for that authenticated Caddy prefix; Caddy
# removes `/saas-admin/listmonk` before forwarding to the one shared runtime.
RUN LISTMONK_ADMIN_BASE_PATH=/saas-admin/listmonk/admin/ LISTMONK_API_ROOT_URL=/saas-admin/listmonk yarn build

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=frontend /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -o listmonk -ldflags="-s -w" ./cmd
RUN go install github.com/knadh/stuffbin/...
RUN mkdir -p /out
RUN /go/bin/stuffbin -a stuff -in listmonk -out /out/listmonk config.toml.sample schema.sql queries:/queries permissions.json static/public:/public static/email-templates frontend/dist:/admin i18n:/i18n

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata shadow su-exec
WORKDIR /listmonk
COPY --from=builder /out/listmonk ./listmonk
COPY config.toml.sample ./config.toml
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 9000
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./listmonk"]
