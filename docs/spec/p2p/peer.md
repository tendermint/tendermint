# Tendermint Peers

This document explains how Tendermint Peers are identified and how they connect to one another.

For details on peer discovery, see the [peer exchange (PEX) reactor doc](https://github.com/tendermint/tendermint/blob/master/docs/spec/reactors/pex/pex.md).

## Peer Identity

Tendermint peers are expected to maintain long-term persistent identities in the form of a public key.
Each peer has an ID defined as `peer.ID == peer.PubKey.Address()`, where `Address` uses the scheme defined in `crypto` package.

A single peer ID can have multiple IP addresses associated with it, but a node
will only ever connect to one at a time.

When attempting to connect to a peer, we use the PeerURL: `<ID>@<IP>:<PORT>`.
We will attempt to connect to the peer at IP:PORT, and verify,
via authenticated encryption, that it is in possession of the private key
corresponding to `<ID>`. This prevents man-in-the-middle attacks on the peer layer.

## Connections

All p2p connections use TCP.
Upon establishing a successful TCP connection with a peer,
two handhsakes are performed: one for authenticated encryption, and one for Tendermint versioning.
Both handshakes have configurable timeouts (they should complete quickly).

### Authenticated Encryption Handshake

Tendermint implements the Station-to-Station protocol
using ED25519 keys for Diffie-Helman key-exchange and NACL SecretBox for encryption.
It goes as follows:
- generate an emphemeral ED25519 keypair
- send the ephemeral public key to the peer
- wait to receive the peer's ephemeral public key
- compute the Diffie-Hellman shared secret using the peers ephemeral public key and our ephemeral private key
- generate two nonces to use for encryption (sending and receiving) as follows:
    - sort the ephemeral public keys in ascending order and concatenate them
    - RIPEMD160 the result
    - append 4 empty bytes (extending the hash to 24-bytes)
    - the result is nonce1
    - flip the last bit of nonce1 to get nonce2
    - if we had the smaller ephemeral pubkey, use nonce1 for receiving, nonce2 for sending;
        else the opposite
- all communications from now on are encrypted using the shared secret and the nonces, where each nonce
increments by 2 every time it is used
- we now have an encrypted channel, but still need to authenticate
- generate a common challenge to sign:
    - SHA256 of the sorted (lowest first) and concatenated ephemeral pub keys
- sign the common challenge with our persistent private key
- send the go-wire encoded persistent pubkey and signature to the peer
- wait to receive the persistent public key and signature from the peer
- verify the signature on the challenge using the peer's persistent public key


If this is an outgoing connection (we dialed the peer) and we used a peer ID,
then finally verify that the peer's persistent public key corresponds to the peer ID we dialed,
ie. `peer.PubKey.Address() == <ID>`.

The connection has now been authenticated. All traffic is encrypted.

Note: only the dialer can authenticate the identity of the peer,
but this is what we care about since when we join the network we wish to
ensure we have reached the intended peer (and are not being MITMd).

### Peer Filter

Before continuing, we check if the new peer has the same ID as ourselves or
an existing peer. If so, we disconnect.

We also check the peer's address and public key against
an optional whitelist which can be managed through the ABCI app -
if the whitelist is enabled and the peer does not qualify, the connection is
terminated.


### Tendermint Version Handshake

The Tendermint Version Handshake allows the peers to exchange their NodeInfo:

```golang
type NodeInfo struct {
  ID         p2p.ID
  ListenAddr string

  Network    string
  Version    string
  Channels   []int8

  Moniker    string
  Other      []string
}
```

The connection is disconnected if:
- `peer.NodeInfo.ID` is not equal `peerConn.ID`
- `peer.NodeInfo.Version` is not formatted as `X.X.X` where X are integers known as Major, Minor, and Revision
- `peer.NodeInfo.Version` Major is not the same as ours
- `peer.NodeInfo.Network` is not the same as ours
- `peer.Channels` does not intersect with our known Channels.
- `peer.NodeInfo.ListenAddr` is malformed or is a DNS host that cannot be
  resolved


At this point, if we have not disconnected, the peer is valid.
It is added to the switch and hence all reactors via the `AddPeer` method.
Note that each reactor may handle multiple channels.

## Connection Activity

Once a peer is added, incoming messages for a given reactor are handled through
that reactor's `Receive` method, and output messages are sent directly by the Reactors
on each peer. A typical reactor maintains per-peer go-routine(s) that handle this.
