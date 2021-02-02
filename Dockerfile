FROM alpine:3.13

WORKDIR /
COPY bin/scheduler /
ENTRYPOINT ["/scheduler"]