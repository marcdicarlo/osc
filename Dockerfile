# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder

WORKDIR /src

# CGO is required by github.com/mattn/go-sqlite3.
RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/osc .

FROM alpine:3.21

WORKDIR /app

# Minimal runtime dependencies.
RUN apk add --no-cache ca-certificates libgcc sqlite3

COPY --from=builder /out/osc /usr/local/bin/osc

# App expects config.yaml in the working directory.
COPY sample.config.yaml /etc/osc/config.yaml

ENTRYPOINT ["/usr/local/bin/osc"]
