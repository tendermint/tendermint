#!/bin/bash

DATA=$GOPATH/src/github.com/tendermint/tendermint/benchmarks/blockchain/data
if [ ! -d $DATA ]; then
  echo "no data found, generating a chain... (this only has to happen once)"

  tendermint init --home $DATA
  cp $DATA/config.toml $DATA/config2.toml
  echo "
  [consensus]
  timeout_commit = 0
  " >> $DATA/config.toml

  echo "starting node"
  tendermint node \
    --home $DATA \
    --proxy_app kvstore \
    --p2p.laddr tcp://127.0.0.1:56656 \
    --rpc.laddr tcp://127.0.0.1:56657 \
    --log_level error &

  echo "making blocks for 60s"
  sleep 60

  mv $DATA/config2.toml $DATA/config.toml

  kill %1

  echo "done generating chain."
fi

# validator node
HOME1=$TMPDIR$RANDOM$RANDOM
cp -R $DATA $HOME1
echo "starting validator node"
tendermint node \
  --home $HOME1 \
  --proxy_app kvstore \
  --p2p.laddr tcp://127.0.0.1:56656 \
  --rpc.laddr tcp://127.0.0.1:56657 \
  --log_level error &
sleep 1

# downloader node
HOME2=$TMPDIR$RANDOM$RANDOM
tendermint init --home $HOME2
cp $HOME1/genesis.json $HOME2
printf "starting downloader node"
tendermint node \
  --home $HOME2 \
  --proxy_app kvstore \
  --p2p.laddr tcp://127.0.0.1:56666 \
  --rpc.laddr tcp://127.0.0.1:56667 \
  --p2p.persistent_peers 127.0.0.1:56656 \
  --log_level error &

# wait for node to start up so we only count time where we are actually syncing
sleep 0.5
while curl localhost:56667/status 2> /dev/null | grep "\"latest_block_height\": 0," > /dev/null
do
  printf '.'
  sleep 0.2
done
echo

echo "syncing blockchain for 10s"
for i in {1..10}
do
  sleep 1
  HEIGHT="$(curl localhost:56667/status 2> /dev/null \
    | grep 'latest_block_height' \
    | grep -o ' [0-9]*' \
    | xargs)"
  let 'RATE = HEIGHT / i'
  echo "height: $HEIGHT, blocks/sec: $RATE"
done

kill %1
kill %2
rm -rf $HOME1 $HOME2
