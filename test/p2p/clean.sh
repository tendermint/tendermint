#! /bin/bash

docker rm -vf $(docker ps -aq)
docker network rm local_testnet
