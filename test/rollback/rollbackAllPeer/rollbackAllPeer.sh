#!/usr/bin/env bash
# usage rollbackAllPeer.sh $rollbackHeight

docker stop $(docker ps -aq)
docker rm $(docker ps -aq)
docker run -tid --net=bridge --name=peer0 -p 26656:26656 -p 26657:26657 -p 26658:26658 -v $PWD/newtm/node0:/chaindata -v /usr/bin:/bin  ubuntu /bin/tendermint node --proxy_app=kvstore --p2p.pex=false --log_level=info --home /chaindata  --rollback_data=true --rollback_height=$1 --rollback_height_flag=true

typeset -l address
address=$(cat $PWD/newtm/node0/config/priv_validator_key.json | jq ".address" |sed 's/\"//g')
echo $address

for((i=1;i<4;i++))
do
	rm -rf $PWD/newtm/node$i/data/cs.wal
    port2=$((26656 + $i * 10))
    port3=$((26657 + $i * 10))
    port4=$((26658 + $i * 10))
    docker run -tid --net=bridge --name=peer$i -p $port2:26656 -p $port3:26657 -p $port4:26658 -v $PWD/newtm/node$i:/chaindata -v /usr/bin:/bin  ubuntu /bin/tendermint node --proxy_app=kvstore --p2p.pex=false --p2p.persistent_peers=$address@172.17.0.1:26656 --log_level=info --home /chaindata  --rollback_data=true --rollback_height=$1 --rollback_height_flag=true
done
