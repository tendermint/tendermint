# Pull base image.
FROM golang:1.4

ENV USER tmuser
ENV DATA_ROOT /data/tendermint

# set user right away for determinism
RUN groupadd -r $USER \
  && useradd -r -s /bin/false -g $USER $USER

# create directory for persistence and give our user ownership
RUN mkdir -p $DATA_ROOT \
  && chown -R $USER:$USER $DATA_ROOT

# Set the env variables to non-interactive
ENV DEBIAN_FRONTEND noninteractive
ENV DEBIAN_PRIORITY critical
ENV DEBCONF_NOWARNINGS yes
ENV TERM linux
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

# grab deps (git)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    git && \
  rm -rf /var/lib/apt/lists/*

# grab deps (node)
RUN curl -sL https://deb.nodesource.com/setup_5.x | bash -
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    nodejs && \
  rm -rf /var/lib/apt/lists/*

# copy run.sh
COPY ./run.sh $DATA_ROOT/run.sh
RUN chmod +x $DATA_ROOT/run.sh

# persist data, set user
WORKDIR $DATA_ROOT
VOLUME $DATA_ROOT
USER $USER
ENV TMROOT $DATA_ROOT

# run tendermint
CMD ["./run.sh"]
