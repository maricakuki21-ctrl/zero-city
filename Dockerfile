FROM node:22.22.2-alpine AS frontend
WORKDIR /src/frontend
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
COPY docs/legal/ /src/docs/legal/
ENV NODE_OPTIONS=--max-old-space-size=4096
RUN pnpm build

FROM golang:1.26.5-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/backend/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -buildvcs=false -p 2 -tags embed -ldflags='-s -w' -o /zero-city ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 city
WORKDIR /app
COPY --from=backend /zero-city /app/zero-city
RUN mkdir /app/data && chown city:city /app/data
USER city
ENV DATA_DIR=/app/data SERVER_HOST=0.0.0.0 SERVER_PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/zero-city"]
