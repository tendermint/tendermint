# Tendermint Peers

This document explains how Tendermint Peers are identified, how they connect to one another,
and how other peers are found.

## Peer Identity

Tendermint peers are expected to maintain long-term persistent identities in the form of a private key.
Each peer has an ID defined as `peer.ID == peer.PrivKey.Address()`, where `Address` uses the scheme defined in go-crypto.

Peer ID's must come with some Proof-of-Work; that is,
they must satisfy `peer.PrivKey.Address() < target` for some difficulty target.
This ensures they are not too easy to generate.

A single peer ID can have multiple IP addresses associated with - for simplicity, we only keep track
of the latest one.

When attempting to connect to a peer, we use the PeerURL: `<ID>@<IP>:<PORT>`.
We will attempt to connect to the peer at IP:PORT, and verify,
via authenticated encryption, that it is in possession of the private key
corresponding to `<ID>`. This prevents man-in-the-middle attacks on the peer layer.

Peers can also be connected to without specifying an ID, ie. `<IP>:<PORT>`.
In this case, the peer cannot be authenticated and other means, such as a VPN,
must be used.

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
- generate nonces to use for encryption
    - TODO
- all communications from now on are encrypted using the shared secret
- generate a common challenge to sign
- sign the common challenge with our persistent private key
- send the signed challenge and persistent public key to the peer
- wait to receive the signed challenge and persistent public key from the peer
- verify the signature in the signed challenge using the peers persistent public key


If this is an outgoing connection (we dialed the peer) and we used a peer ID,
then finally verify that the `peer.PubKey` corresponds to the peer ID we dialed,
ie. `peer.PubKey.Address() == <ID>`.

The connection has now been authenticated. All traffic is encrypted.

Note that only the dialer can authenticate the identity of the peer,
but this is what we care about since when we join the network we wish to
ensure we have reached the intended peer (and are not being MITMd).


### Peer Filter

Before continuing, we check if the new peer has the same ID has ourselves or
an existing peer. If so, we disconnect.

We also check the peer's address and public key against
an optional whitelist which can be managed through the ABCI app -
if the whitelist is enabled and the peer is not on it, the connection is
terminated.


### Tendermint Version Handshake

The Tendermint Version Handshake allows the peers to exchange their NodeInfo, which contains:

```
type NodeInfo struct {
	PubKey     crypto.PubKey `json:"pub_key"`
	Moniker    string        `json:"moniker"`
	Network    string        `json:"network"`
	RemoteAddr string        `json:"remote_addr"`
	ListenAddr string        `json:"listen_addr"` // accepting in
	Version    string        `json:"version"` // major.minor.revision
    Channels   []int8        `json:"channels"` // active reactor channels
	Other      []string      `json:"other"`   // other application specific data
}
```

The connection is disconnected if:
- `peer.NodeInfo.PubKey != peer.PubKey`
- `peer.NodeInfo.Version` is not formatted as `X.X.X` where X are integers known as Major, Minor, and Revision
- `peer.NodeInfo.Version` Major is not the same as ours
- `peer.NodeInfo.Version` Minor is not the same as ours
- `peer.NodeInfo.Network` is not the same as ours


At this point, if we have not disconnected, the peer is valid and added to the switch,
so it is added to all reactors.


### Connection Activity

