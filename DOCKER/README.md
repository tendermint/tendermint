# Build and run a docker image and container

```
# Build base Docker image
# Make sure ./run.sh exists.
docker build -t tendermint/tmbase -f Dockerfile .

# Log into dockerhub
docker login

# Push latest build to dockerhub
docker push tendermint/tmbase
```
