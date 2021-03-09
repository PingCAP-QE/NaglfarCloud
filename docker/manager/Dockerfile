# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /src
COPY . .

RUN rm -rf /go/src/
RUN --mount=type=cache,id=naglfar_go_pkg,target=/go/pkg \
    --mount=type=cache,id=naglfar_go_cache,target=/root/.cache/go-build \
    --mount=type=tmpfs,id=naglfar_go_src,target=/go/src/ make clean && make webhook

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /src/bin/webhook manager
USER nonroot:nonroot

ENTRYPOINT ["/manager"]
