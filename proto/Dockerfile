# This Dockerfile defines an image containing tools for linting, formatting,
# and compiling the Tendermint protos.
FROM golang:1.17-alpine

# Install a commonly used set of programs for use with our protos.
# clang-extra-tools is included here because it provides clang-format,
# used to format the .proto files.
RUN apk add --no-cache build-base clang-extra-tools curl git

ENV GOLANG_PROTOBUF_VERSION=1.3.1 \
    GOGO_PROTOBUF_VERSION=1.3.2

# Retrieve the go protoc programs and copy them into the PATH
RUN go install github.com/golang/protobuf/protoc-gen-go@v${GOLANG_PROTOBUF_VERSION} && \
  go install github.com/gogo/protobuf/protoc-gen-gogo@v${GOGO_PROTOBUF_VERSION} && \
  go install github.com/gogo/protobuf/protoc-gen-gogofaster@v${GOGO_PROTOBUF_VERSION} && \
  mv "$(go env GOPATH)"/bin/* /usr/local/bin/

# Copy the 'buf' program out of the buildbuf/buf container.
COPY --from=bufbuild/buf:latest /usr/local/bin/* /usr/local/bin/
