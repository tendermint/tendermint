# This Dockerfile defines a toolbox for use in linting, formatting, and compiling the Tendermint protos.

FROM bufbuild/buf:latest as buf

FROM alpine:3.14

# Install a commonly used set of programs for use with our protos.
# clang-extra-tools is included here because it provides clang-format, the program used for formatting the protos.
RUN echo 'http://dl-cdn.alpinelinux.org/alpine/edge/testing' >> /etc/apk/repositories && \
  apk add --update --no-cache build-base clang-extra-tools curl git go && \
  rm -rf /var/cache/apk/*

ENV GOLANG_PROTOBUF_VERSION=1.3.1 \
  GOGO_PROTOBUF_VERSION=1.3.2

# Retrieve the go protoc programs and copy them into the PATH
RUN GO111MODULE=on go get \
  github.com/golang/protobuf/protoc-gen-go@v${GOLANG_PROTOBUF_VERSION} \
  github.com/gogo/protobuf/protoc-gen-gogo@v${GOGO_PROTOBUF_VERSION} \
  github.com/gogo/protobuf/protoc-gen-gogofaster@v${GOGO_PROTOBUF_VERSION} && \
  mv "$(go env GOPATH)"/bin/* /usr/local/bin/ 

# Copy the 'buf' program out of the buildbuf/buf container.
COPY --from=buf /usr/local/bin/* /usr/local/bin/
