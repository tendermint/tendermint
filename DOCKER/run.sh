#! /bin/bash

if [[ $BARAK_SEED ]]; then 
	cat ./cmd/barak/$BARAK_SEED | ./build/barak &
fi

tendermint node
