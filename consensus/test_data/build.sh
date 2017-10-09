#!/usr/bin/env bash

# XXX: removes tendermint dir
# TODO: does not work on OSX

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/../.." && pwd )"

# Change into that dir because we expect that.
cd "$DIR" || exit

# Make sure we have a tendermint command.
if ! hash tendermint 2>/dev/null; then
    make install
fi

# Make sure we have a cutWALUntil binary.
if ! hash ./scripts/cutWALUntil/cutWALUntil 2>/dev/null; then
	cd ./scripts/cutWALUntil/ && go build && cd - || exit
fi

# specify a dir to copy
# TODO: eventually we should replace with `tendermint init --test`
DIR_TO_COPY=$HOME/.tendermint_test/consensus_state_test

TMHOME="$HOME/.tendermint"
#rm -rf "$TMHOME"
#cp -r "$DIR_TO_COPY" "$TMHOME"
#mv $TMHOME/config.toml $TMHOME/config.toml.bak
cp $TMHOME/genesis.json $TMHOME/genesis.json.bak

function reset(){
	tendermint unsafe_reset_all
	cp $TMHOME/genesis.json.bak $TMHOME/genesis.json
}

reset

# empty block
function empty_block(){
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 5
	killall tendermint

	./scripts/cutWALUntil/cutWALUntil ~/.tendermint/data/cs.wal/wal 1 consensus/test_data/new_empty_block.cswal
	mv consensus/test_data/new_empty_block.cswal consensus/test_data/empty_block.cswal

	reset
}

# many blocks
function many_blocks(){
	bash scripts/txs/random.sh 1000 36657 &> /dev/null &
	PID=$!
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 10
	killall tendermint
	kill -9 $PID

	./scripts/cutWALUntil/cutWALUntil ~/.tendermint/data/cs.wal/wal 6 consensus/test_data/new_many_blocks.cswal
	mv consensus/test_data/new_many_blocks.cswal consensus/test_data/many_blocks.cswal

	reset
}


# small block 1
function small_block1(){
	bash scripts/txs/random.sh 1000 36657 &> /dev/null &
	PID=$!
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 10
	killall tendermint
	kill -9 $PID

	./scripts/cutWALUntil/cutWALUntil ~/.tendermint/data/cs.wal/wal 1 consensus/test_data/new_small_block1.cswal
	mv consensus/test_data/new_small_block1.cswal consensus/test_data/small_block1.cswal

	reset
}


# small block 2 (part size = 64)
function small_block2(){
	cat ~/.tendermint/genesis.json | jq '. + {consensus_params: {block_size_params: {max_bytes: 22020096}, block_gossip_params: {block_part_size_bytes: 512}}}' > ~/.tendermint/new_genesis.json
	mv ~/.tendermint/new_genesis.json ~/.tendermint/genesis.json
	bash scripts/txs/random.sh 1000 36657 &> /dev/null &
	PID=$!
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 5
	killall tendermint
	kill -9 $PID

	./scripts/cutWALUntil/cutWALUntil ~/.tendermint/data/cs.wal/wal 1 consensus/test_data/new_small_block2.cswal
	mv consensus/test_data/new_small_block2.cswal consensus/test_data/small_block2.cswal

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
