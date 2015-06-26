
# Persistence 

It's good practice to use a data-only container, alongside the main application.
The `docker.sh` script sets it all up for you, and provides the
same functionality as `-v host_dir:image_dir` but by copying the data rather than
mounting it.

# To Play

The commands should work from tendermint/tendermint or tendermint/tendermint/DOCKER,
save the removal of DOCKER from the path.

Get quickly caught up with the testnet: `FAST_SYNC=true ./DOCKER/docker.sh`

Use a pre-existing `~/.tendermint`: `VC=~/.tendermint NO_BUILD=true ./DOCKER/docker.sh`

This is like doing `-v ~/.tendermint:/data/tendermint`, but better.

Use `NO_BUILD` to avoid waiting if the image is already built. 

Rerunning `docker.sh` will require you to delete the old containers:

`docker rm mint mintdata`

However, if you remove the `mintdata` container, you delete the data (the blockchain).
If you don't use the `VC` option, your key will be deleted too

To avoid deleting and recreating the data container, use

`VD=true NO_BUILD=true ./DOCKER/docker.sh`

Of course, once running, you can just control the main container with `docker stop mint` and `docker start mint`
