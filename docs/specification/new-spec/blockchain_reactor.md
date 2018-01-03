# Blockchain Reactor

The Blockchain Reactor's high level responsibility is to maintain connection to a reasonable number
of peers in the network, request blocks from them or provide them with blocks, validate and persist
the blocks to disk and play blocks to the ABCI app.

## Block Reactor

* coordinates synching with other peers

## Block Pool

* maintain connections to other peers

## Block Store

* persists blocks to disk
