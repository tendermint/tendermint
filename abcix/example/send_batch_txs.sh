#!/bin/bash

set -u;

for i in {1..20}; do
		curl -s 'localhost:26657/broadcast_tx_commit?tx="key'$i'=val'$i',junjiah,'$i'"' > /dev/null &
done

wait
