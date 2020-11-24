---
order: 6
---

# Configure a Light Client

Tendermint comes with a built-in `tendermint light` command, which can be used
to run a light client proxy server, verifying Tendermint RPC. All calls that
can be tracked back to a block header by a proof will be verified before
passing them back to the caller. Other than that, it will present the same
interface as a full Tendermint node.

You can start the light client proxy server by running `tendermint light <chainID>`,
with a variety of flags to specify the primary node,  the witness nodes (which cross-check
the information provided by the primary), the hash and height of the trusted header,
and more.

For example:

```bash
$ tendermint light supernova -p tcp://233.123.0.140:26657 \
  -w tcp://179.63.29.15:26657,tcp://144.165.223.135:26657 \
  --height=10 --hash=37E9A6DD3FA25E83B22C18835401E8E56088D0D7ABC6FD99FCDC920DD76C1C57
```

For additional options, run `tendermint light --help`.

## Where to obtain trusted height & hash

One way to obtain a semi-trusted hash & height is to query multiple full nodes
and compare their hashes:

```bash
$ curl -s https://233.123.0.140:26657:26657/commit | jq "{height: .result.signed_header.header.height, hash: .result.signed_header.commit.block_id.hash}"
{
  "height": "273",
  "hash": "188F4F36CBCD2C91B57509BBF231C777E79B52EE3E0D90D06B1A25EB16E6E23D"
}
```
