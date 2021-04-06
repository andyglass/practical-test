FROM golang:1.16-alpine as builder

ENV GOOS=linux \
    GARCH=amd64 \
    CGO_ENABLED=0 \
    GO111MODULE=on

WORKDIR /workspace

COPY go.mod go.sum *.go ./

RUN apk update && \
    apk add --update --no-cache ca-certificates && \
    go mod download && \
    go mod verify && \
    go build -x -v -a  -o /build/gcs-cp .


FROM alpine:latest

COPY --from=builder /build/gcs-cp /usr/local/bin/

ENTRYPOINT ["gcs-cp"]
CMD ["-help"]