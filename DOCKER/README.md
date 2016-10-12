# Docker

Tendermint uses docker for deployment of testnets via the [mintnet](github.com/tendermint/mintnet) tool. 

For faster development iterations (ie. to avoid docker builds), 
the dockerfile just sets up the OS, and tendermint is fetched/installed at runtime.

For the deterministic docker builds used in testing, see the [tests directory](https://github.com/tendermint/tendermint/tree/master/test)

# Build and run a docker image and container

These are notes for the dev team. 

```
# Build base Docker image
# Make sure ./run.sh exists.
docker build -t tendermint/tmbase -f Dockerfile .

# Log into dockerhub
docker login

# Push latest build to dockerhub
docker push tendermint/tmbase
```
