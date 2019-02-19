#!/usr/bin/env bash
# usage rollbackOnePeer.sh $rollbackHeight i.e rollbackOnePeer.sh 200

docker stop peer3
docker rm peer3

typeset -l address
address=$(cat $PWD/newtm/node0/config/priv_validator_key.json | jq ".address" |sed 's/\"//g')
echo $address

rm -rf $PWD/newtm/node3/data/cs.wal
tendermint rollback --home="$PWD/newtm/node3/" --rollback_data=true --rollback_height=$1 --rollback_height_flag=true
docker run -tid --net=bridge --name=peer3 -p 26686:26656 -p 26687:26657 -p 26688:26658 -v $PWD/newtm/node3:/chaindata -v /usr/bin:/bin  ubuntu /bin/tendermint node --proxy_app=kvstore --p2p.persistent_peers=$address@172.17.0.1:46656 --log_level=info --home /chaindata
