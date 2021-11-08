FROM golang:1.16-alpine

ENV LD_LIBRARY_PATH=/usr/local/lib

RUN apk add bash build-base bzip2-dev gflags-dev linux-headers \
            perl snappy-dev util-linux wget zlib-dev zstd-dev

# Install cleveldb
RUN \
  wget -q https://github.com/google/leveldb/archive/v1.20.tar.gz \
  && tar xvf v1.20.tar.gz \
  && cd leveldb-1.20 \
  && make \
  && cp -a out-static/lib* out-shared/lib* /usr/local/lib \
  && cd include \
  && cp -a leveldb /usr/local/include \
  && ldconfig $LD_LIBRARY_PATH \
  && cd ../.. \
  && rm -rf v1.20.tar.gz leveldb-1.20

# Install Rocksdb
RUN \
  wget -q https://github.com/facebook/rocksdb/archive/v6.6.4.tar.gz \
  && tar -zxf v6.6.4.tar.gz \
  && cd rocksdb-6.6.4 \
  && sed -i'' 's/install -C/install -c/g' Makefile \
  && DEBUG_LEVEL=0 make -j4 shared_lib \
  && make install-shared \
  && ldconfig $LD_LIBRARY_PATH \
  && cd .. \
  && rm -rf v6.6.4.tar.gz rocksdb-6.6.4
