#! /bin/bash

cd $GOPATH/src/github.com/tendermint/tendermint

# specify a dir to copy
# NOTE: eventually we should replace with `tendermint init --test`
DIR=$HOME/.tendermint_test/consensus_state_test

# XXX: remove tendermint dir
rm -rf $HOME/.tendermint
cp -r $DIR $HOME/.tendermint

function reset(){
	rm -rf $HOME/.tendermint/data
	tendermint unsafe_reset_priv_validator
}

reset

# empty block
tendermint node --proxy_app=dummy &> /dev/null &
sleep 5
killall tendermint

# /q would print up to and including the match, then quit. 
# /Q doesn't include the match. 
# http://unix.stackexchange.com/questions/11305/grep-show-all-the-file-up-to-the-match
sed '/HEIGHT: 2/Q' ~/.tendermint/data/cs.wal/wal  > consensus/test_data/empty_block.cswal

reset

# small block 1
bash scripts/txs/random.sh 1000 36657 &> /dev/null &
PID=$!
tendermint node --proxy_app=dummy &> /dev/null &
sleep 5
killall tendermint
kill -9 $PID

sed '/HEIGHT: 2/Q' ~/.tendermint/data/cs.wal/wal  > consensus/test_data/small_block1.cswal

reset


# small block 2 (part size = 512)
echo "" >> ~/.tendermint/config.toml
echo "block_part_size = 512" >> ~/.tendermint/config.toml
bash scripts/txs/random.sh 1000 36657 &> /dev/null &
PID=$!
tendermint node --proxy_app=dummy &> /dev/null &
sleep 5
killall tendermint
kill -9 $PID

sed '/HEIGHT: 2/Q' ~/.tendermint/data/cs.wal/wal  > consensus/test_data/small_block2.cswal

reset

