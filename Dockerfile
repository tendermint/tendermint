# Pull base image.
FROM golang:1.4.2-wheezy

# Set the env variables to non-interactive
ENV DEBIAN_FRONTEND noninteractive
ENV DEBIAN_PRIORITY critical
ENV DEBCONF_NOWARNINGS yes
ENV TERM linux
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    libgmp3-dev && \
  rm -rf /var/lib/apt/lists/*

# Install go
# ADD tendermint user
RUN useradd tendermint

# Get rid of tendermint user login shell
RUN usermod -s /sbin/nologin tendermint

ADD . /go/src/github.com/tendermint/tendermint
WORKDIR /go/src/github.com/tendermint/tendermint
RUN make

# Set environment variables
USER tendermint
ENV USER tendermint
ENV TMROOT /tendermint_root
# docker run -v $(pwd)/tendermint_root:/tendermint_root
CMD [ "./build/tendermint", "node" ]
