# Docker

Tendermint uses docker for deployment of testnets via the [mintnet](github.com/tendermint/mintnet) tool.

For the deterministic docker builds used in testing, see the [tests directory](https://github.com/tendermint/tendermint/tree/master/test)

# Build and run a docker image and container

These are notes for the dev team.

```
# Build base Docker image
docker build -t "tendermint/tendermint" -t "tendermint/tendermint:0.8.0" -t "tendermint/tendermint:0.8" .

# Log into dockerhub
docker login

# Push latest build to dockerhub
docker push tendermint/tendermint
```
