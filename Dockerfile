# syntax = docker/dockerfile:1.0-experimental
FROM golang:alpine3.10
RUN apk add --no-cache gcc libc-dev make bash git

WORKDIR /src
COPY . .

RUN rm -rf /go/src/
RUN --mount=type=cache,id=tipocket_go_pkg,target=/go/pkg \
    --mount=type=cache,id=tipocket_go_cache,target=/root/.cache/go-build \
    --mount=type=tmpfs,id=tipocket_go_src,target=/go/src/ make clean && make scheduler

FROM alpine:3.10

COPY --from=0 src/bin/scheduler /bin/kube-scheduler
WORKDIR /bin
CMD ["kube-scheduler"]
