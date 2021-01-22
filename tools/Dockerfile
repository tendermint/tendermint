FROM golang

ENV LD_LIBRARY_PATH=/usr/local/lib

RUN apt-get update && apt-get install -y --no-install-recommends \
  libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev libzstd-dev make tar wget

# Install cleveldb
RUN \
  wget https://github.com/google/leveldb/archive/v1.20.tar.gz \
  && tar xvf v1.20.tar.gz \
  && cd leveldb-1.20 \
  && make \
  && cp -a out-static/lib* out-shared/lib* /usr/local/lib \
  && cd include \
  && cp -a leveldb /usr/local/include \
  && ldconfig \
  && cd ../.. \
  && rm -rf v1.20.tar.gz leveldb-1.20

# Install Rocksdb
RUN \
  wget https://github.com/facebook/rocksdb/archive/v6.6.4.tar.gz \
  && tar -zxf v6.6.4.tar.gz \
  && cd rocksdb-6.6.4 \
  && DEBUG_LEVEL=0 make -j4 shared_lib \
  && make install-shared \
  && ldconfig \
  && cd .. \
  && rm -rf v6.6.4.tar.gz rocksdb-6.6.4
