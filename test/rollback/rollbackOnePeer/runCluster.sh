#!/usr/bin/env bash

cd ../../../cmd/tendermint && go build && mv tendermint /usr/bin
cd ../../test/rollback/rollbackAllPeer


tendermint testnet --v=4
mv mytestnet newtm

typeset -l address
address=$(cat $PWD/newtm/node0/config/priv_validator_key.json | jq ".address" |sed 's/\"//g')
echo $address
echo "{\"priv_key\":"$(cat $PWD/newtm/node0/config/priv_validator_key.json | jq ".priv_key")"}"  >$PWD/newtm/node0/config/node_key.json

echo "docker run -tid --net=bridge --name=peer0 -p 26656:26656 -p 26657:26657 -p 26658:26658 -v $PWD/newtm/node0:/chaindata -v /usr/bin:/bin  ubuntu /bin/tendermint node --proxy_app=kvstore --p2p.pex=false --log_level=info --home /chaindata" > startNode.sh

for((i=1;i<4;i++))
do
    port2=$((26656 + $i * 10))
    port3=$((26657 + $i * 10))
    port4=$((26658 + $i * 10))
    port6=$((26656 + $i * 10 - 10))
    echo "" >>startNode.sh
    echo "docker run -tid --net=bridge --name=peer$i -p $port2:26656 -p $port3:26657 -p $port4:26658 -v $PWD/newtm/node$i:/chaindata -v /usr/bin:/bin  ubuntu /bin/tendermint node --proxy_app=kvstore --p2p.pex=false --p2p.persistent_peers=$address@172.17.0.1:26656 --log_level=info --home /chaindata" >> startNode.sh
done

bash startNode.sh
