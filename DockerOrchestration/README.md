# Orchestration with docker-compose
Please ensure that you have installed the latest version of [docker and docker-compose](https://www.docker.com/community-edition) for your platform.

To run a cluster of 4 validator nodes, switch into the ```DockerOrchestration``` directory and run ```docker-compose up```.
This will build 8 containers. It will build 4 validator containers with their private keys and a common genesis file as well
as 4 ABCI app nodes that are linked to one of the validator containers.

In order to use your own ABCI app with this setup, just modify the dockerfile inside the ABCIApp folder to build and run
your own app. The rest should work as usual.

**This is not production ready as it uses publicily known private keys.**

## TODO
- Enable users to specify a number of core nodes and then automatically handle the creation of keys, adding them to the genesis.json file and networking them
- Enable this for automatic deploy with docker swarm in production environments