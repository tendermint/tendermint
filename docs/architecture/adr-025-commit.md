# ADR 025 Commit

## Context

Currently the `Commit` structure contains a lot of potentially redundant or unnecessary data.
It contains a list of precommits from every validator, where the precommit
includes the whole `Vote` structure. Thus each of the commit height, round,
type, and blockID are repeated for every validator, and could be deduplicated.

```
type Commit struct {
    BlockID    BlockID `json:"block_id"`
    Precommits []*Vote `json:"precommits"`
}

type Vote struct {
    ValidatorAddress Address   `json:"validator_address"`
    ValidatorIndex   int       `json:"validator_index"`
    Height           int64     `json:"height"`
    Round            int       `json:"round"`
    Timestamp        time.Time `json:"timestamp"`
    Type             byte      `json:"type"`
    BlockID          BlockID   `json:"block_id"`
    Signature        []byte    `json:"signature"`
}
```
References:
[#1648](https://github.com/tendermint/tendermint/issues/1648)
[#2179](https://github.com/tendermint/tendermint/issues/2179)
[#2226](https://github.com/tendermint/tendermint/issues/2226)

## Proposed Solution

We can improve efficiency by replacing the usage of the `Vote` struct with a subset of each vote, and by storing the constant values (`Height`, `Round`, `BlockID`) in the Commit itself.

```
type Commit struct {
    Height  int64
    Round   int
    BlockID    BlockID      `json:"block_id"`
    Precommits []*CommitSig `json:"precommits"`
}

type CommitSig struct {
    BlockID  BlockIDFlag
    ValidatorAddress Address
    Timestamp time.Time
    Signature []byte
}


// indicate which BlockID the signature is for
type BlockIDFlag int

const (
	BlockIDFlagAbsent BlockIDFlag = iota // vote is not included in the Commit.Precommits
	BlockIDFlagCommit                    // voted for the Commit.BlockID
	BlockIDFlagNil                       // voted for nil
)

```

Note the need for an extra byte to indicate whether the signature is for the BlockID or for nil.
This byte can also be used to indicate an absent vote, rather than using a nil object like we currently do,
which has been [problematic for compatibility between Amino and proto3](https://github.com/tendermint/go-amino/issues/260).

Note we also continue to store the `ValidatorAddress` in the `CommitSig`.
While this still takes 20-bytes per signature, it ensures that the Commit has all
information necessary to reconstruct Vote, which simplifies mapping between Commit and Vote objects
and with debugging. It also may be necessary for the light-client to know which address a signature corresponds to if
it is trying to verify a current commit with an older validtor set.

## Status

Proposed

## Consequences

### Positive

Removing the Type/Height/Round/Index and the BlockID saves roughly 80 bytes per precommit.
It varies because some integers are varint. The BlockID contains two 32-byte hashes an integer,
and the Height is 8-bytes.

For a chain with 100 validators, that's up to 8kB in savings per block!


### Negative

- Large breaking change to the block and commit structure
- Requires differentiating in code between the Vote and CommitSig objects, which may add some complexity (votes need to be reconstructed to be verified and gossiped)

### Neutral

- Commit.Precommits no longer contains nil values
