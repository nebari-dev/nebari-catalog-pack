# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.26 AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Regenerate templ components, then build a static binary.
RUN go run github.com/a-h/templ/cmd/templ@v0.3.857 generate
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/catalog ./cmd/catalog

# ---- runtime ----
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/catalog /catalog
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/catalog"]
