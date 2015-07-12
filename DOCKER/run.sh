#! /bin/bash

if [[ $BARAK_SEED ]]; then 
	cat ./cmd/barak/$BARAK_SEED | ./build/barak &
fi

if [ "$FAST_SYNC" = "true" ]; then
	tendermint node --fast_sync
else 
	tendermint node
fi

