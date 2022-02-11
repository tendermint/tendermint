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
   - Within in the issue members from both the Tendermint-go and Tendermint-rs team will leave comments. If there is not consensus on the change an [RFC](../rfc/README.md) may be requested.
  1a. Submission of an RFC as a pull request should be made to facilitate further discussion.
  1b. Merge the RFC.
2. Make the necessary changes to the `.proto` file(s), [core data structures](../spec/core/data_structures.md) and/or [ABCI protocol](../spec/abci/apps.md).
3. Open issues within Tendermint-go and Tendermint-rs repos. This is used to notify the teams that a change occurred in the spec.
   1. Tag the issue with a spec version label. This will notify the team the changed has been made on master but has not entered a release.