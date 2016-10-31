# Pull base image.
FROM golang

ENV USER tmuser
ENV DATA_ROOT /data/tendermint

# Set user right away for determinism
RUN groupadd -r $USER \
  && useradd -r -s /bin/false -g $USER $USER

# Create home directory for USER
# Needed for nodejs/nom
RUN mkdir -p /home/$USER \
  && chown -R $USER:$USER /home/$USER

# Create directory for persistence and give our user ownership
RUN mkdir -p $DATA_ROOT \
  && chown -R $USER:$USER $DATA_ROOT

# Set the env variables to non-interactive
ENV DEBIAN_FRONTEND noninteractive
ENV DEBIAN_PRIORITY critical
ENV DEBCONF_NOWARNINGS yes
ENV TERM linux
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

# Grab deps (git)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    git && \
  rm -rf /var/lib/apt/lists/*

# Grab deps (node)
RUN curl -sL https://deb.nodesource.com/setup_7.x | bash -
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    nodejs && \
  rm -rf /var/lib/apt/lists/*

# Copy run.sh
COPY ./run.sh $DATA_ROOT/run.sh
RUN chmod +x $DATA_ROOT/run.sh

# Persist data, set user
WORKDIR $DATA_ROOT
VOLUME $DATA_ROOT
USER $USER
ENV TMROOT $DATA_ROOT

EXPOSE 46656
EXPOSE 46657

# Run tendermint
CMD ["./run.sh"]
