# Creating a proposal

A block consists of a header, transactions, votes (the commit),
and a list of evidence of malfeasance (ie. signing conflicting votes).

We include no more than 1/10th of the maximum block size
(`ConsensusParams.Block.MaxBytes`) of evidence with each block.

## Reaping transactions from the mempool

When we reap transactions from the mempool, we calculate maximum data
size by subtracting maximum header size (`MaxHeaderBytes`), the maximum
amino overhead for a block (`MaxAminoOverheadForBlock`), the size of
the last commit (if present) and evidence (if present). While reaping
we account for amino overhead for each transaction.

```go
func MaxDataBytes(maxBytes int64, valsCount, evidenceCount int) int64 {
	return maxBytes -
		MaxAminoOverheadForBlock -
		MaxHeaderBytes -
		int64(valsCount)*MaxVoteBytes -
		int64(evidenceCount)*MaxEvidenceBytes
}
```

## Validating transactions in the mempool

Before we accept a transaction in the mempool, we check if it's size is no more
than {MaxDataSize}. {MaxDataSize} is calculated using the same formula as
above, except because the evidence size is unknown at the moment, we subtract
maximum evidence size (1/10th of the maximum block size).

```go
func MaxDataBytesUnknownEvidence(maxBytes int64, valsCount int) int64 {
	return maxBytes -
		MaxAminoOverheadForBlock -
		MaxHeaderBytes -
		int64(valsCount)*MaxVoteBytes -
		MaxEvidenceBytesPerBlock(maxBytes)
}
```
