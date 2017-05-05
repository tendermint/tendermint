# Tendermint P2P Tests

These scripts facilitate setting up and testing a local testnet using docker containers.

Setup your own local testnet as follows.

For consistency, we assume all commands are run from the Tendermint repository root (ie. $GOPATH/src/github.com/tendermint/tendermint).

First, build the docker image:

```
docker build -t tendermint_tester -f ./test/docker/Dockerfile .
```

Now create the docker network:

```
docker network create --driver bridge --subnet 172.57.0.0/16 my_testnet
```

This gives us a new network with IP addresses in the rage `172.57.0.0 - 172.57.255.255`.
Peers on the network can have any IP address in this range. 
For our four node network, let's pick `172.57.0.101 - 172.57.0.104`.
Since we use Tendermint's default listening port of 46656, our list of seed nodes will look like:

```
172.57.0.101:46656,172.57.0.102:46656,172.57.0.103:46656,172.57.0.104:46656
```

Now we can start up the peers. We already have config files setup in `test/p2p/data/`.
Let's use a for-loop to start our peers:

```
for i in $(seq 1 4); do
	docker run -d \
	  --net=my_testnet\
	  --ip="172.57.0.$((100 + $i))" \
	  --name local_testnet_$i \
	  --entrypoint tendermint \
	  -e TMHOME=/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$i/core \
	  tendermint_tester node --p2p.seeds 172.57.0.101:46656,172.57.0.102:46656,172.57.0.103:46656,172.57.0.104:46656 --proxy_app=dummy
done
```

If you now run `docker ps`, you'll see your containers!

We can confirm they are making blocks by checking the `/status` message using `curl` and `jq` to pretty print the output json:

```
curl 172.57.0.101:46657/status | jq . 
```



