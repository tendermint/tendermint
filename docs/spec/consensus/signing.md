# Validator Signing

Here we specify the rules for validating a proposal and vote before signing.
First we include some general notes on validating data structures common to both types.
We then provide specific validation rules for each. Finally, we include validation rules to prevent double-sigining.

## SignedMsgType

The `SignedMsgType` is a single byte that refers to the type of the message
being signed. It is defined in Go as follows:

```
// SignedMsgType is a type of signed message in the consensus.
type SignedMsgType byte

const (
	// Votes
	PrevoteType   SignedMsgType = 0x01
	PrecommitType SignedMsgType = 0x02

	// Proposals
	ProposalType SignedMsgType = 0x20
)
```

All signed messages must correspond to one of these types.

## Timestamp

Timestamp validation is subtle and there are currently no bounds placed on the
timestamp included in a proposal or vote. It is expected that validators will honestly
report their local clock time. The median of all timestamps
included in a commit is used as the timestamp for the next block height.

Timestamps are expected to be strictly monotonic for a given validator, though
this is not currently enforced.

## ChainID

ChainID is an unstructured string with a max length of 50-bytes.
In the future, the ChainID may become structured, and may take on longer lengths.
For now, it is recommended that signers be configured for a particular ChainID,
and to only sign votes and proposals corresponding to that ChainID.

## BlockID

BlockID is the structure used to represent the block:

```
type BlockID struct {
	Hash        []byte
	PartsHeader PartSetHeader
}

type PartSetHeader struct {
	Hash  []byte
	Total int
}
```

To be included in a valid vote or proposal, BlockID must either represent a `nil` block, or a complete one.
We introduce two methods, `BlockID.IsZero()` and `BlockID.IsComplete()` for these cases, respectively.

`BlockID.IsZero()` returns true for BlockID `b` if each of the following
are true:

```
b.Hash == nil
b.PartsHeader.Total == 0
b.PartsHeader.Hash == nil
```

`BlockID.IsComplete()` returns true for BlockID `b` if each of the following
are true:

```
len(b.Hash) == 32
b.PartsHeader.Total > 0
len(b.PartsHeader.Hash) == 32
```

## Proposals

The structure of a proposal for signing looks like:

```
type CanonicalProposal struct {
	Type      SignedMsgType // type alias for byte
	Height    int64         `binary:"fixed64"`
	Round     int64         `binary:"fixed64"`
	POLRound  int64         `binary:"fixed64"`
	BlockID   BlockID
	Timestamp time.Time
	ChainID   string
}
```

A proposal is valid if each of the following lines evaluates to true for proposal `p`:

```
p.Type == 0x20
p.Height > 0
p.Round >= 0
p.POLRound >= -1
p.BlockID.IsComplete()
```

In other words, a proposal is valid for signing if it contains the type of a Proposal
(0x20), has a positive, non-zero height, a
non-negative round, a POLRound not less than -1, and a complete BlockID.

## Votes

The structure of a vote for signing looks like:

```
type CanonicalVote struct {
	Type      SignedMsgType // type alias for byte
	Height    int64         `binary:"fixed64"`
	Round     int64         `binary:"fixed64"`
	BlockID   BlockID
	Timestamp time.Time
	ChainID   string
}
```

A vote is valid if each of the following lines evaluates to true for vote `v`:

```
v.Type == 0x1 || v.Type == 0x2
v.Height > 0
v.Round >= 0
v.BlockID.IsZero() || v.BlockID.IsComplete()
```

In other words, a vote is valid for signing if it contains the type of a Prevote
or Precommit (0x1 or 0x2, respectively), has a positive, non-zero height, a
non-negative round, and an empty or valid BlockID.

## Invalid Votes and Proposals

Votes and proposals which do not satisfy the above rules are considered invalid.
Peers gossipping invalid votes and proposals may be disconnected from other peers on the network.
Note, however, that there is not currently any explicit mechanism to punish validators signing votes or proposals that fail
these basic validation rules.

## Double Signing

Signers must be careful not to sign conflicting messages, also known as "double signing" or "equivocating".
Tendermint has mechanisms to publish evidence of validators that signed conflicting votes, so they can be punished
by the application. Note Tendermint does not currently handle evidence of conflciting proposals, though it may in the future.

### State

To prevent such double signing, signers must track the height, round, and type of the last message signed.
Assume the signer keeps the following state, `s`:

```
type LastSigned struct {
	Height	int64
	Round	int64
	Type	SignedMsgType // byte
}
```

After signing a vote or proposal `m`, the signer sets:

```
s.Height = m.Height
s.Round = m.Round
s.Type = m.Type
```

### Proposals

A signer should only sign a proposal `p` if any of the following lines are true:

```
p.Height > s.Height
p.Height == s.Height && p.Round > s.Round
```

In other words, a proposal should only be signed if it's at a higher height, or a higher round for the same height.
Once a proposal or vote has been signed for a given height and round, a proposal should never be signed for the same height and round.

### Votes

A signer should only sign a vote `v` if any of the following lines are true:

```
v.Height > s.Height
v.Height == s.Height && v.Round > s.Round
v.Height == s.Height && v.Round == s.Round && v.Step == 0x1 && s.Step == 0x20
v.Height == s.Height && v.Round == s.Round && v.Step == 0x2 && s.Step != 0x2
```

In other words, a vote should only be signed if it's:

- at a higher height
- at a higher round for the same height
- a prevote for the same height and round where we haven't signed a prevote or precommit (but have signed a proposal)
- a precommit for the same height and round where we haven't signed a precommit (but have signed a proposal and/or a prevote)

This means that once a validator signs a prevote for a given height and round, the only other message it can sign for that height and round is a precommit.
And once a validator signs a precommit for a given height and round, it must not sign any other message for that same height and round.
