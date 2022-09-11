FROM bufbuild/buf:latest as buf

FROM golang:1.14-alpine3.11 as builder

RUN apk add --update --no-cache build-base curl git upx && \
  rm -rf /var/cache/apk/*

ENV GOLANG_PROTOBUF_VERSION=1.3.1 \
  GOGOPROTO_VERSION=1.4.1

RUN GO111MODULE=on go get \
  github.com/golang/protobuf/protoc-gen-go@v${GOLANG_PROTOBUF_VERSION} \
  github.com/cosmos/gogoproto/protoc-gen-gogo@v${GOGOPROTO_VERSION} \
  github.com/cosmos/gogoproto/protoc-gen-gogofaster@v${GOGOPROTO_VERSION} && \
  mv /go/bin/protoc-gen-go* /usr/local/bin/


FROM alpine:edge

WORKDIR /work

RUN echo 'http://dl-cdn.alpinelinux.org/alpine/edge/testing' >> /etc/apk/repositories && \
  apk add --update --no-cache clang && \
  rm -rf /var/cache/apk/*

COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=buf /usr/local/bin /usr/local/bin
