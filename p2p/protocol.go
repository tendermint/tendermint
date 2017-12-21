package p2p

// Protocol defines the libp2p protocol that we will use for the tendermint
// multiplexed connection (mconn) on the libp2p network.
// Streams are multiplexed and their protocol tag helps
// libp2p handle them to the right handler functions.
const Protocol = "/tendermint/0.0.1"
