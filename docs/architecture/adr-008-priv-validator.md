# ADR 008: PrivValidator

## Context

The current PrivValidator is monolithic and isn't easily reuseable by alternative signers.

For instance, see https://github.com/tendermint/tendermint/issues/673

The goal is to have a clean PrivValidator interface like:

```
type PrivValidator interface {
	Address() data.Bytes
	PubKey() crypto.PubKey

	SignVote(chainID string, vote *types.Vote) error
	SignProposal(chainID string, proposal *types.Proposal) error
	SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error
}
```

It should also be easy to re-use the LastSignedInfo logic to avoid double signing.

## Decision

Tendermint node's should support only two in-process PrivValidator implementations:

- PrivValidatorUnencrypted uses an unencrypted private key in a "priv_validator.json" file - no configuration required (just `tendermint init`).
- PrivValidatorSocket uses a socket to send signing requests to another process - user is responsible for starting that process themselves.

The PrivValidatorSocket address can be provided via flags at the command line -
doing so will cause Tendermint to ignore any "priv_validator.json" file and to listen
on the given address for incoming connections from an external priv_validator process.
It will halt any operation until at least one external process succesfully
connected.

The external priv_validator process will dial the address to connect to Tendermint,
and then Tendermint will send requests on the ensuing connection to sign votes and proposals.
Thus the external process initiates the connection, but the Tendermint process makes all requests.
In a later stage we're going to support multiple validators for fault
tolerance. To prevent double signing they need to be synced, which is deferred
to an external solution (see #1185).

In addition, Tendermint will provide implementations that can be run in that external process.
These include:

- PrivValidatorEncrypted uses an encrypted private key persisted to disk - user must enter password to decrypt key when process is started.
- PrivValidatorLedger uses a Ledger Nano S to handle all signing.

What follows are descriptions of useful types

### Signer

```
type Signer interface {
	Sign(msg []byte) (crypto.Signature, error)
}
```

Signer signs a message. It can also return an error.

### ValidatorID


ValidatorID is just the Address and PubKey

```
type ValidatorID struct {
	Address data.Bytes    `json:"address"`
	PubKey  crypto.PubKey `json:"pub_key"`
}
```

### LastSignedInfo

LastSignedInfo tracks the last thing we signed:

```
type LastSignedInfo struct {
	Height    int64            `json:"height"`
	Round     int              `json:"round"`
	Step      int8             `json:"step"`
	Signature crypto.Signature `json:"signature,omitempty"` // so we dont lose signatures
	SignBytes data.Bytes       `json:"signbytes,omitempty"` // so we dont lose signatures
}
```

It exposes methods for signing votes and proposals using a `Signer`.

This allows it to easily be reused by developers implemented their own PrivValidator.

### PrivValidatorUnencrypted

```
type PrivValidatorUnencrypted struct {
	ID             types.ValidatorID `json:"id"`
	PrivKey        PrivKey           `json:"priv_key"`
	LastSignedInfo *LastSignedInfo   `json:"last_signed_info"`
}
```

Has the same structure as currently, but broken up into sub structs.

Note the LastSignedInfo is mutated in place every time we sign.

### PrivValidatorJSON

The "priv_validator.json" file supports only the PrivValidatorUnencrypted type.

It unmarshals into PrivValidatorJSON, which is used as the default PrivValidator type.
It wraps the PrivValidatorUnencrypted and persists it to disk after every signature.

## Status

Accepted.

## Consequences

### Positive

- Cleaner separation of components enabling re-use.

### Negative

- More files - led to creation of new directory.

### Neutral

