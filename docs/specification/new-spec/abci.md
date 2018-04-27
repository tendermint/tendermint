# Application Blockchain Interface (ABCI)

ABCI is the interface between Tendermint (a state-machine replication engine)
and an application (the actual state machine).

The ABCI message types are defined in a [protobuf
file](https://github.com/tendermint/abci/blob/master/types/types.proto).
For full details on the ABCI message types and protocol, see the [ABCI
specificaiton](https://github.com/tendermint/abci/blob/master/specification.rst).
For additional details on server implementation, see the [ABCI
readme](https://github.com/tendermint/abci#implementation).

Here we provide some more details around the use of ABCI by Tendermint and
clarify common "gotchas".

## Validator Updates

Updates to the Tendermint validator set can be made by returning `Validator`
objects in the `ResponseBeginBlock`:

```
message Validator {
    bytes pub_key = 1;
    int64 power = 2;
}
```

The `pub_key` is the Amino encoded public key for the validator. For details on
Amino encoded public keys, see the [section of the encoding spec](./encoding.md#public-key-cryptography).

For Ed25519 pubkeys, the Amino prefix is always "1624DE6220". For example, the 32-byte Ed25519 pubkey
`76852933A4686A721442E931A8415F62F5F1AEDF4910F1F252FB393F74C40C85` would be
Amino encoded as
`1624DE622076852933A4686A721442E931A8415F62F5F1AEDF4910F1F252FB393F74C40C85`

(Note: in old versions of Tendermint (pre-v0.19.0), the pubkey is just prefixed with a
single type byte, so for ED25519 we'd have `pub_key = 0x1 | pub`)

The `power` is the new voting power for the validator, with the
following rules:

- power must be non-negative
- if power is 0, the validator must already exist, and will be removed from the
  validator set
- if power is non-0:
    - if the validator does not already exist, it will be added to the validator
      set with the given power
    - if the validator does already exist, its power will be adjusted to the given power

## Query

Query is a generic message type with lots of flexibility to enable diverse sets
of queries from applications. Tendermint has no requirements from the Query
message for normal operation - that is, the ABCI app developer need not implement Query functionality if they do not wish too.
That said, Tendermint makes a number of queries to support some optional
features. These are:

### Peer Filtering

When Tendermint connects to a peer, it sends two queries to the ABCI application
using the following paths, with no additional data:

 - `/p2p/filter/addr/<IP:PORT>`, where `<IP:PORT>` denote the IP address and
   the port of the connection
 - `p2p/filter/pubkey/<ID>`, where `<ID>` is the peer node ID (ie. the
   pubkey.Address() for the peer's PubKey)

If either of these queries return a non-zero ABCI code, Tendermint will refuse
to connect to the peer.
