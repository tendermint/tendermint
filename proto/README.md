# Protocol Buffers

This sections defines the types and messages shared across implementations. The definition of the data structures are located in the [core/data_structures](../spec/core/data_structures.md) for the core data types and ABCI definitions are located in the [ABCI](../spec/abci/README.md) section.

## Process of Updates

The `.proto` files within this section are core to the protocol and updates must be treated as such.

### Steps

1. Make an issue with the proposed change.
   - Within in the issue members from both the Tendermint-go and Tendermint-rs team will leave comments. If there is not consensus on the change an [RFC](../rfc/README.md) may be requested.
  1a. Submission of an RFC as a pull request should be made to facilitate further discussion.
  1b. Merge the RFC.
2. Make the necessary changes to the `.proto` file(s), [core data structures](../spec/core/data_structures.md) and/or [ABCI protocol](../spec/abci/apps.md).
3. Open issues within Tendermint-go and Tendermint-rs repos. This is used to notify the teams that a change occurred in the spec.
   1. Tag the issue with a spec version label. This will notify the team the changed has been made on master but has not entered a release.

### Versioning

The spec repo aims to be versioned. Once it has been versioned, updates to the protobuf files will live on master. After a certain amount of time, decided on by Tendermint-go and Tendermint-rs team leads, a release will be made on the spec repo. The spec may contain minor releases as well, depending on the implementation these changes may lead to a breaking change. If so, the implementation team should open an issue within the spec repo requiring a major release of the spec.

If the steps above were followed each implementation should have issues tagged with a spec change label. Once all issues have been completed the team should signify their readiness for release.
