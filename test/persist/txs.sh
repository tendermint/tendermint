#! /bin/bash
set -u

# wait till node is up, send txs
ADDR=$1 #="127.0.0.1:26657"
curl -s $ADDR/status > /dev/null
ERR=$?
while [ "$ERR" != 0 ]; do
	sleep 1	
	curl -s $ADDR/status > /dev/null
	ERR=$?
done

# send a bunch of txs over a few blocks
echo "Node is up, sending txs"
for i in $(seq 1 5); do
	for _ in $(seq 1 100); do
		tx=$(head -c 8 /dev/urandom | hexdump -ve '1/1 "%.2X"')
		curl -s "$ADDR/broadcast_tx_async?tx=0x$tx" &> /dev/null
	done
	echo "sent 100"
	sleep 1
done
