# Pull base image.
FROM golang:1.4.2-wheezy

ENV user tendermint
ENV data_root /data/tendermint

# set user right away for determinism
RUN groupadd -r $user \
  && useradd -r -s /bin/false -g $user $user

# create directory for persistence and give our user ownership
RUN mkdir -p $data_root \
  && chown -R $user:$user $data_root

# Set the env variables to non-interactive
ENV DEBIAN_FRONTEND noninteractive
ENV DEBIAN_PRIORITY critical
ENV DEBCONF_NOWARNINGS yes
ENV TERM linux
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

# grab deps (gmp)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    libgmp3-dev && \
  rm -rf /var/lib/apt/lists/*

# set the repo and install tendermint
ENV repo /go/src/github.com/tendermint/tendermint
ADD . $repo
WORKDIR $repo
RUN make 

# persist data, set user
VOLUME $data_root
USER tendermint
ENV TMROOT $data_root

# run tendermint
CMD ["./DOCKER/run.sh"]

