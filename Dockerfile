# syntax=docker/dockerfile:1

# ---- frontend build ----
FROM node:20 AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- go build ----
FROM golang:1.26 AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Bring in the built SPA so it is embedded by the web package.
COPY --from=web /web/dist ./web/dist

RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/catalog ./cmd/catalog

# ---- runtime ----
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/catalog /catalog
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/catalog"]
