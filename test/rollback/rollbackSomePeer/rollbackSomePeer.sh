#!/usr/bin/env bash
# usage rollbackSomePeer.sh $rollbackHeight i.e rollbackSomePeer.sh 200

docker stop peer1 && docker stop peer2
docker rm peer1 && docker rm peer2

# get the address of peer0
typeset -l address
address=$(cat $PWD/newtm/node0/config/priv_validator_key.json | jq ".address" |sed 's/\"//g')
echo $address

## rollback data and restart node
for((i=1;i<3;i++))
do
	tendermint rollback --home="$PWD/newtm/node$i/" --rollback_data=true --rollback_height=$1 --rollback_height_flag=false
	rm -rf $PWD/newtm/node$i/data/cs.wal
    port2=$((26656 + $i * 10))
    port3=$((26657 + $i * 10))
    port4=$((26658 + $i * 10))
    docker run -tid --net=bridge --name=peer$i -p $port2:26656 -p $port3:26657 -p $port4:26658 -v $PWD/newtm/node$i:/chaindata -v /usr/bin:/bin  ubuntu /bin/tendermint node --proxy_app=kvstore --p2p.pex=false --p2p.persistent_peers=$address@172.17.0.1:26656 --log_level=info --home /chaindata
done
