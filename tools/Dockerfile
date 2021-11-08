# This file defines the container image used to build and test tm-db in CI.
# The CI workflows use the latest tag of tendermintdev/docker-tm-db-testing
# built from these settings.
#
# The jobs defined in the Build & Push workflow will build and update the image
# when changes to this file are merged.  If you have other changes that require
# updates here, merge the changes here first and let the image get updated (or
# push a new version manually) before PRs that depend on them.

FROM golang:1.17-bullseye AS build

ENV LD_LIBRARY_PATH=/usr/local/lib

RUN apt-get update && apt-get install -y --no-install-recommends \
    libbz2-dev libgflags-dev libsnappy-dev libzstd-dev zlib1g-dev \
    make tar wget

FROM build AS install
ARG LEVELDB=1.20
ARG ROCKSDB=6.24.2

# Install cleveldb
RUN \
  wget -q https://github.com/google/leveldb/archive/v${LEVELDB}.tar.gz \
  && tar xvf v${LEVELDB}.tar.gz \
  && cd leveldb-${LEVELDB} \
  && make \
  && cp -a out-static/lib* out-shared/lib* /usr/local/lib \
  && cd include \
  && cp -a leveldb /usr/local/include \
  && ldconfig \
  && cd ../.. \
  && rm -rf v${LEVELDB}.tar.gz leveldb-${LEVELDB}

# Install Rocksdb
RUN \
  wget -q https://github.com/facebook/rocksdb/archive/v${ROCKSDB}.tar.gz \
  && tar -zxf v${ROCKSDB}.tar.gz \
  && cd rocksdb-${ROCKSDB} \
  && DEBUG_LEVEL=0 make -j4 shared_lib \
  && make install-shared \
  && ldconfig \
  && cd .. \
  && rm -rf v${ROCKSDB}.tar.gz rocksdb-${ROCKSDB}
