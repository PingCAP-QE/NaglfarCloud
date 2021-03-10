# syntax = docker/dockerfile:1.0-experimental
# Build the manager binary
FROM golang:1.16-alpine as builder

WORKDIR /src
COPY . .

RUN rm -rf /go/src/
RUN --mount=type=cache,id=naglfar_go_pkg,target=/go/pkg \
    --mount=type=cache,id=naglfar_go_cache,target=/root/.cache/go-build \
    --mount=type=tmpfs,id=naglfar_go_src,target=/go/src/ \
    go build -o bin/webhook webhook/main.go

FROM alpine
WORKDIR /
COPY --from=builder /src/bin/webhook manager
