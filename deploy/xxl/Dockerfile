FROM node:22-bookworm AS frontend

WORKDIR /src
COPY frontend ./frontend
RUN corepack enable && cd frontend && yarn install --frozen-lockfile
RUN cd frontend/email-builder && yarn install --frozen-lockfile && yarn build
RUN mkdir -p frontend/public/static/email-builder && cp -R frontend/email-builder/dist/. frontend/public/static/email-builder/
RUN cd frontend && yarn build

FROM golang:1.26.1-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
COPY --from=frontend /src/frontend/public/static/email-builder ./frontend/public/static/email-builder
RUN go install github.com/knadh/stuffbin/...
RUN CGO_ENABLED=0 go build -o listmonk -ldflags="-s -w" ./cmd
RUN "$(go env GOPATH)/bin/stuffbin" -a stuff -in listmonk -out listmonk \
    config.toml.sample \
    schema.sql \
    queries:/queries \
    permissions.json \
    static/public:/public \
    static/email-templates \
    frontend/dist:/admin \
    i18n:/i18n

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata shadow su-exec
WORKDIR /listmonk
COPY --from=build /src/listmonk ./listmonk
COPY config.toml.sample ./config.toml
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
EXPOSE 9000
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./listmonk"]
