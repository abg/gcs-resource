FROM golang:alpine as builder

WORKDIR /gcs-resource

COPY . /gcs-resource

RUN CGO_ENABLED=0 GOBIN=/assets go install -trimpath -ldflags="-s -w" github.com/frodenas/gcs-resource/cmd/...

FROM alpine:edge AS resource
COPY --from=builder assets/ /opt/resource/
RUN apk add --no-cache bash tzdata ca-certificates unzip zip gzip tar