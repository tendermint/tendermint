# ADR 064: Batch Verification

## Changelog

- January 28, 2021: Created (@marbar3778)

## Context

Tendermint uses public private key cryptography for validator signing. When a block is proposed and voted on validators sign a message representing acceptance or rejection of a block. These signatures are also used to verify previous blocks are correct if a node is syncing. Currently, Tendermint requires each signature to be verified individually, this leads to a slow down of block times.

## Alternative Approaches

- Signature aggregation
  - Signature aggregation is an alternative to batch verification. Signature aggregation leads to fast verification and smaller block sizes. At the time of writing this ADR there is on going work to enable signature aggregation in Tendermint. The reason why we have opted to not introduce it at this time is because it will slow down verification times. One of the two benefits can be done, aggregation, but verification would suffer because the software would need to verify each signature individually due to their uniqueness, then aggregate them. For example if we were to implement signature aggregation with BLS, there could be a potential slow down of 10x-100x in verification speeds.

## Decision

Adopt Batch Verification.

## Detailed Design

A new interface will be introduced. This interface will have three methods `NewBatchVerifier`, `Add` and `VerifyBatch`.

```go
type BatchVerification interface {
  NewBatchVerifier() BatchVerification // NewBatchVerifier create a new verifier where keys, signatures and messages can be added as entries
  Add(key crypto.Pubkey, signature, message []byte) bool // Add appends an entry into the BatchVerifier.
  VerifyBatch() bool // VerifyBatch verifies all the entries in the BatchVerifier. If the verification fails it is unknown which entry failed and each entry will need to be verified individually.
}
```

- `NewBatchVerifier` creates a new verifier. This verifier will be populated with entries to be verified. 
- `Add` adds an entry to the Verifier. Add accepts a public key and two slice of bytes (signature and message). 
- `VerifyBatch` verifies all the entires. At the end of VerifyBatch if the underlying API does not reset the Verifier to its initial state (empty), it should be done here. This prevents accidentally reusing the verifier with entries from a previous verification.

Above there is mention of an entry. An entry can be constructed in many ways depending on the needs of the underlying curve. A simple approach would be:

```go
type entry struct {
  pubKey crypto.Pubkey
  signature []byte
  message []byte
}
```

The main reason this approach is being taken is to prevent simple mistakes. Some APIs allow the user to create three slices and pass them to the `VerifyBatch` function but this relies on the user to safely generate all the slices (see example below). We would like to minimize the possibility of making a mistake.

```go
func VerifyBatch(keys []crypto.Pubkey, signatures, messages[][]byte) bool
```

This change will not require will not effect any users in anyway other than faster verification times.

This new api will be used for verification in both consensus and block syncing. Within the current Verify functions there will be a check to see if the key types supports the BatchVerification API. If it does it will execute batch verification, if not single signature verification will be used. 

If batch verifications fails for any reason, it will not be known which entry caused the failure. Verification will need to revert to single signature verification.

Starting out only ed25519 will support batch verification. 

## Status

Proposed
## Consequences

- If verification fails it could make verification slower than single signature verification.

### Positive

- Faster verification times, if the curve supports it

### Negative

- No way to see which key failed verification
  - A failure means reverting back to single signature verification.

### Neutral

## References

> Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given design choice? If so link them here!

- {reference link}
