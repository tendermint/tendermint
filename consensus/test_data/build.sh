#! /bin/bash

# XXX: removes tendermint dir

cd $GOPATH/src/github.com/tendermint/tendermint

# specify a dir to copy
# TODO: eventually we should replace with `tendermint init --test`
DIR=$HOME/.tendermint_test/consensus_state_test

rm -rf $HOME/.tendermint
cp -r $DIR $HOME/.tendermint

function reset(){
	rm -rf $HOME/.tendermint/data
	tendermint unsafe_reset_priv_validator
}

reset

# empty block
function empty_block(){
tendermint node --proxy_app=dummy &> /dev/null &
sleep 5
killall tendermint

# /q would print up to and including the match, then quit. 
# /Q doesn't include the match. 
# http://unix.stackexchange.com/questions/11305/grep-show-all-the-file-up-to-the-match
sed '/HEIGHT: 2/Q' ~/.tendermint/data/cs.wal/wal  > consensus/test_data/empty_block.cswal

reset
}

# many blocks
function many_blocks(){
bash scripts/txs/random.sh 1000 36657 &> /dev/null &
PID=$!
tendermint node --proxy_app=dummy &> /dev/null &
sleep 7
killall tendermint
kill -9 $PID

sed '/HEIGHT: 7/Q' ~/.tendermint/data/cs.wal/wal  > consensus/test_data/many_blocks.cswal

reset
}


# small block 1
function small_block1(){
bash scripts/txs/random.sh 1000 36657 &> /dev/null &
PID=$!
tendermint node --proxy_app=dummy &> /dev/null &
sleep 10
killall tendermint
kill -9 $PID

sed '/HEIGHT: 2/Q' ~/.tendermint/data/cs.wal/wal  > consensus/test_data/small_block1.cswal

reset
}


# small block 2 (part size = 512)
function small_block2(){
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
}



case "$1" in 
	"small_block1")
		small_block1
		;;
	"small_block2")
		small_block2
		;;
	"empty_block")
		empty_block
		;;
	"many_blocks")
		many_blocks
		;;
	*)
		small_block1
		small_block2
		empty_block
		many_blocks
esac


