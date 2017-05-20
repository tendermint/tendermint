#! /bin/bash

if [[ "$SEEDS" != "" ]]; then
	SEEDS_FLAG="--seeds=$SEEDS"
fi

./dummy --persist .tendermint/data/dummy_data >> app.log 2>&1 &
./tendermint node --log_level=info $SEEDS_FLAG >> tendermint.log 2>&1 &
