# Build the manager binary
FROM golang:1.16-alpine as builder

WORKDIR /src
COPY . .

RUN go build -o bin/webhook webhook/main.go

FROM alpine
WORKDIR /
COPY --from=builder /src/bin/webhook manager
