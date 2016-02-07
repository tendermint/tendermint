#! /bin/bash
set -e

# assumes machines already created
N_MACHINES=4

MACH_PREFIX=mach

TESTNET_DIR=~/testnets_mach
CHAINS_AND_VALS=$TESTNET_DIR/chains_and_vals.json
CHAINS_DIR=$TESTNET_DIR/chains
VALS_DIR=$TESTNET_DIR/validators

VALSETS=(validator-set-numero-uno BOA BunkBankBandaloo victory_validators)
#VALSETS=(my-val-set)

CHAINS=(blockchain1 chainiac Chainelle chain-a-daisy blockchain100 bandit-chain gambit-chain gambit-chain-duo gambit-chain-1002)
#CHAINS=(my-chain)

mkdir -p $TESTNET_DIR
echo "{}" > $CHAINS_AND_VALS

echo "Make some validator sets"
# make some validator sets
for valset in ${VALSETS[@]}; do
        mintnet init validator-set $VALS_DIR/$valset
        netmon chains-and-vals val $CHAINS_AND_VALS $VALS_DIR/$valset
done


echo "Make some blockchains"
# make some blockchains with each validator set
for i in ${!CHAINS[@]}; do
	valset=$(($i % ${#VALSETS[@]}))
        mintnet init --machines "${MACH_PREFIX}[1-4]" chain --validator-set $VALS_DIR/${VALSETS[$valset]} $CHAINS_DIR/${CHAINS[$i]}
done


echo "Start the chains"
for chain in ${CHAINS[@]}; do
	# randomize the machine order for each chain
	machs=`python -c "import random; x=range(1, $(($N_MACHINES+1))); random.shuffle(x); print \",\".join(map(str,x))"` 
        echo $machs
        echo $chain
        mintnet start --publish-all --machines ${MACH_PREFIX}[$machs] app-$chain $CHAINS_DIR/$chain

	# add the new chain config
	netmon chains-and-vals chain $CHAINS_AND_VALS $CHAINS_DIR/$chain
done
