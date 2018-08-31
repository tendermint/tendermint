# ADR 020: Limiting txs size inside a block

## Changelog

13-08-2018: Initial Draft
15-08-2018: Second version after Dev's comments
28-08-2018: Third version after Ethan's comments
30-08-2018: AminoOverheadForBlock => MaxAminoOverheadForBlock
31-08-2018: Bounding evidence and chain ID

## Context

We currently use MaxTxs to reap txs from the mempool when proposing a block,
but enforce MaxBytes when unmarshalling a block, so we could easily propose a
block thats too large to be valid.

We should just remove MaxTxs all together and stick with MaxBytes, and have a
`mempool.ReapMaxBytes`.

But we can't just reap BlockSize.MaxBytes, since MaxBytes is for the entire block,
not for the txs inside the block. There's extra amino overhead + the actual
headers on top of the actual transactions + evidence + last commit.

## Proposed solution

Therefore, we should

1) Get rid of MaxTxs.
2) Rename MaxTxsBytes to MaxBytes.

When we need to ReapMaxBytes from the mempool, we calculate the upper bound as follows:

```
ExactLastCommitBytes = {number of validators currently enabled} * {MaxVoteBytes}
MaxEvidenceBytesPerBlock = MaxBytes / 10
ExactEvidenceBytes = cs.evpool.PendingEvidence(MaxEvidenceBytesPerBlock) * MaxEvidenceBytes

mempool.ReapMaxBytes(MaxBytes - MaxAminoOverheadForBlock - ExactLastCommitBytes - ExactEvidenceBytes - MaxHeaderBytes)
```

where MaxVoteBytes, MaxEvidenceBytes, MaxHeaderBytes and MaxAminoOverheadForBlock
are constants defined inside the `types` package:

- MaxVoteBytes - 170 bytes
- MaxEvidenceBytes - 364 bytes
- MaxHeaderBytes - 476 bytes (~276 bytes hashes + 200 bytes - 50 UTF-8 encoded
  symbols of chain ID 4 bytes each in the worst case + amino overhead)
- MaxAminoOverheadForBlock - 8 bytes (assuming MaxHeaderBytes includes amino
  overhead for encoding header, MaxVoteBytes - for encoding vote, etc.)

ChainID needs to bound to 50 symbols max.

When reaping evidence, we use MaxBytes to calculate the upper bound (e.g. 1/10)
to save some space for transactions.

NOTE while reaping the `max int` bytes in mempool, we should account that every
transaction will take `len(tx)+aminoOverhead`, where aminoOverhead=1-4 bytes.

We should write a test that fails if the underlying structs got changed, but
MaxXXX stayed the same.

## Status

Proposed.

## Consequences

### Positive

* one way to limit the size of a block
* less variables to configure

### Negative

* constants that need to be adjusted if the underlying structs got changed

### Neutral
