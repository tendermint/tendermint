# Protocol Buffers

This sections defines the protocol buffers used in Tendermint. This is split into two directories: `spec`, the types required for all implementations and `tendermint`, a set of types internal to the Go implementation. All generated go code is also stored in `tendermint`.
More descriptions of the data structures are located in the spec directory as follows:

- [Block](../spec/core/data_structures.md)
- [ABCI](../spec/abci/README.md)
- [P2P](../spec/p2p/messages/README.md)

## Process to generate protos

The `.proto` files within this section are core to the protocol and updates must be treated as such.

### Steps

1. Make an issue with the proposed change.
   - Within the issue members, from the Tendermint team will leave comments. If there is not consensus on the change an [RFC](../docs/rfc/README.md) may be requested.
  1a. Submission of an RFC as a pull request should be made to facilitate further discussion.
  1b. Merge the RFC.
2. Make the necessary changes to the `.proto` file(s), [core data structures](../spec/core/data_structures.md) and/or [ABCI protocol](../spec/abci/apps.md).
3. Rebuild the Go protocol buffers by running `make proto-gen`. Ensure that the project builds correctly by running `make build`.
