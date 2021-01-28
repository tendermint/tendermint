# ADR 064: Batch Verification

## Changelog

- January 28, 2021: Created (@marbar3778)

## Context

Tendermint uses public private key cryptography for validator signing. When a block is proposed and voted on validators sign a message representing acceptance or rejection of a block. These signatures are also used to verify previous blocks are correct if a node is syncing. To be able to proceed with new blocks a threshold of 2/3 validators must vote `yes`, but we must verify the signatures are correct. Currently, Tendermint requires each signature to be verified individually, this leads to a slow down of block times.

## Alternative Approaches

- Signature aggregation
  - Signature aggregation is an alternative to batch verification. Signature aggregation leads to fast verification and smaller block sizes. At the time of writing this ADR there is on going work to enable signature aggregation in Tendermint. The reason why we have opted to not introduce it at this time is because it will slow down verification times. One of the two benefits can be done, aggregation, but verification would suffer because the software would need to verify each signature individually and then aggregate them. For example if we were to implement signature aggregation with BLS, there could be a potential slow down of 10x-100x.

## Decision

Adopt Batch Verification.

## Detailed Design

```go
type BatchVerification interface {
  NewBatchVerifier() BatchVerification // NewBatchVerifier create a new verifier where keys, signatures and messages can be added as entries
  Add(key crypto.Pubkey, signature, message []byte) bool // Add appends an entry into the BatchVerifier.
  VerifyBatch() bool // VerifyBatch verifies all the entries in the BatchVerifier. If the verification fails it is unknown which entry failed and each entry will need to be verified individually.
}
```

> This section does not need to be filled in at the start of the ADR, but must be completed prior to the merging of the implementation.
>
> Here are some common questions that get answered as part of the detailed design:
>
> - What are the user requirements?
>
> - What systems will be affected?
>
> - What new data structures are needed, what data structures will be changed?
>
> - What new APIs will be needed, what APIs will be changed?
>
> - What are the efficiency considerations (time/space)?
>
> - What are the expected access patterns (load/throughput)?
>
> - Are there any logging, monitoring or observability needs?
>
> - Are there any security considerations?
>
> - Are there any privacy considerations?
>
> - How will the changes be tested?
>
> - If the change is large, how will the changes be broken up for ease of review?
>
> - Will these changes require a breaking (major) release?
>
> - Does this change require coordination with the SDK or other?

## Status

> A decision may be "proposed" if it hasn't been agreed upon yet, or "accepted" once it is agreed upon. Once the ADR has been implemented mark the ADR as "implemented". If a later ADR changes or reverses a decision, it may be marked as "deprecated" or "superseded" with a reference to its replacement.

{Deprecated|Proposed|Accepted|Declined}

## Consequences

> This section describes the consequences, after applying the decision. All consequences should be summarized here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References

> Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given design choice? If so link them here!

- {reference link}
