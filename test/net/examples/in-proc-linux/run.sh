#! /bin/bash

if [[ "$SEEDS" != "" ]]; then
	SEEDS_FLAG="--seeds=$SEEDS"
fi

./tendermint node --proxy_app=dummy --log_level=note $SEEDS_FLAG >> tendermint.log 2>&1 &
