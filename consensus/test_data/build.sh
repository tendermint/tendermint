#!/usr/bin/env bash

# Requires: killall command and jq JSON processor.

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/../.." && pwd )"

# Change into that dir because we expect that.
cd "$DIR" || exit 1

# Make sure we have a tendermint command.
if ! hash tendermint 2>/dev/null; then
	make install
fi

# Make sure we have a cutWALUntil binary.
cutWALUntil=./scripts/cutWALUntil/cutWALUntil
cutWALUntilDir=$(dirname $cutWALUntil)
if ! hash $cutWALUntil 2>/dev/null; then
	cd "$cutWALUntilDir" && go build && cd - || exit 1
fi

TMHOME=$(mktemp -d)
export TMHOME="$TMHOME"

if [[ ! -d "$TMHOME" ]]; then
	echo "Could not create temp directory"
  exit 1
else
	echo "TMHOME: ${TMHOME}"
fi

# TODO: eventually we should replace with `tendermint init --test`
DIR_TO_COPY=$HOME/.tendermint_test/consensus_state_test
if [ ! -d "$DIR_TO_COPY" ]; then
    echo "$DIR_TO_COPY does not exist. Please run: go test ./consensus"
    exit 1
fi
echo "==> Copying ${DIR_TO_COPY} to ${TMHOME} directory..."
cp -r "$DIR_TO_COPY"/* "$TMHOME"

# preserve original genesis file because later it will be modified (see small_block2)
cp "$TMHOME/genesis.json" "$TMHOME/genesis.json.bak"

function reset(){
	echo "==> Resetting tendermint..."
	tendermint unsafe_reset_all
	cp "$TMHOME/genesis.json.bak" "$TMHOME/genesis.json"
}

reset

function empty_block(){
	echo "==> Starting tendermint..."
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 5
	echo "==> Killing tendermint..."
	killall tendermint

	echo "==> Copying WAL log..."
	$cutWALUntil "$TMHOME/data/cs.wal/wal" 1 consensus/test_data/new_empty_block.cswal
	mv consensus/test_data/new_empty_block.cswal consensus/test_data/empty_block.cswal

	reset
}

function many_blocks(){
	bash scripts/txs/random.sh 1000 36657 &> /dev/null &
	PID=$!
	echo "==> Starting tendermint..."
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 10
	echo "==> Killing tendermint..."
	kill -9 $PID
	killall tendermint

	echo "==> Copying WAL log..."
	$cutWALUntil "$TMHOME/data/cs.wal/wal" 6 consensus/test_data/new_many_blocks.cswal
	mv consensus/test_data/new_many_blocks.cswal consensus/test_data/many_blocks.cswal

	reset
}


function small_block1(){
	bash scripts/txs/random.sh 1000 36657 &> /dev/null &
	PID=$!
	echo "==> Starting tendermint..."
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 10
	echo "==> Killing tendermint..."
	kill -9 $PID
	killall tendermint

	echo "==> Copying WAL log..."
	$cutWALUntil "$TMHOME/data/cs.wal/wal" 1 consensus/test_data/new_small_block1.cswal
	mv consensus/test_data/new_small_block1.cswal consensus/test_data/small_block1.cswal

	reset
}


# block part size = 512
function small_block2(){
	cat "$TMHOME/genesis.json" | jq '. + {consensus_params: {block_size_params: {max_bytes: 22020096}, block_gossip_params: {block_part_size_bytes: 512}}}' > "$TMHOME/new_genesis.json"
	mv "$TMHOME/new_genesis.json" "$TMHOME/genesis.json"
	bash scripts/txs/random.sh 1000 36657 &> /dev/null &
	PID=$!
	echo "==> Starting tendermint..."
	tendermint node --proxy_app=persistent_dummy &> /dev/null &
	sleep 5
	echo "==> Killing tendermint..."
	kill -9 $PID
	killall tendermint

	echo "==> Copying WAL log..."
	$cutWALUntil "$TMHOME/data/cs.wal/wal" 1 consensus/test_data/new_small_block2.cswal
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

echo "==> Cleaning up..."
rm -rf "$TMHOME"
