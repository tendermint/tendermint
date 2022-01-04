# ADR 74: Migrate Timeout Parameters to Consensus Parameters

## Changelog

- 03-Jan-2022: Initial draft (@williambanfield)

## Status

Proposed

## Context

### Background

Tendermint's consensus timeout parameters are currently configured locally by each validator
in the validator's [config.toml][config-toml].
This means that the validators on a Tendermint network may have different timeouts
from each other. There is no reason for validators on the same network to configure
different timeout values. Proper functioning of the Tendermint consensus algorithm
relies on these parameters being uniform across validators.

The configurable values are as follows:

* `TimeoutPropose`
	* How long the consensus algorithm waits for a proposal block before issuing a prevote.
	* If no prevote arrives by `TimeoutPropose`, then the consensus algorithm will issue a nil prevote.
* `TimeoutProposeDelta`
	* How much the `TimeoutPropose` grows each round.
* `TimeoutPrevote`
	* How long the consensus algorithm waits after receiving +2/3 prevotes with
	no quorum for a value before issuing a precommit for nil.
	(See the [arXiv paper][arxiv-paper], Algorithm 1, Line 34)
* `TimeoutPrevoteDelta`
	* How much the `TimeoutPrevote` increases with each round.
* `TimeoutPrecommit`
	* How long the consensus algorithm waits after receiving +2/3 precommits that
	do not have a quorum for a value before entering the next round. 
	(See the [arXiv paper][arxiv-paper], Algorithm 1, Line 47)
* `TimeoutPrecommitDelta`
	* How much the `TimeoutPrecommit` increases with each round.
* `TimeoutCommit`
	* How long the consensus algorithm waits after committing a block but before starting the new height.
	* This gives a validator a chance to receive slow precommits.
* `SkipTimeoutCommit`
	* Make progress as soon as the node has 100% of the precommits.


### Overview of Change

We will consolidate the timeout parameters and migrate them from the node-local 
`config.toml` file into the network-global consensus parameters.

The 8 timeout parameters will be consolidated down to 6. These will be as follows:

* `TimeoutPropose`
	* Same as current `TimeoutPropose`.
* `TimeoutProposeDelta`
	* Same as current `TimeoutProposeDelta`.
* `TimeoutVote`
	* How long validators wait for votes in both the prevote
	 and precommit phase of the consensus algorithm. This parameter subsumes
	 the current `TimeoutPrevote` and `TimeoutPrecommit` parameters.
* `TimeoutVoteDelta`
	* How much the `TimeoutVote` will grow each successive round.
	 This parameter subsumes the current `TimeoutPrevoteDelta` and `TimeoutPrecommitDelta`
	 parameters.
* `TimeoutCommit`
	* Same as current `TimeoutCommit`.
* `SkipTimeoutCommit`
	* Same as current `SkipTimeoutCommit`.

A safe default will be provided by Tendermint for each of these parameters and
networks will be able to update the parameters as they see fit. Local updates
to these parameters will no longer be possible; instead, the application will control
updating the parameters. Applications using the Cosmos SDK will be automatically be
able to change the values of these consensus parameters [via a governance proposal][cosmos-sdk-consensus-params].

This change is low-risk. While parameters are locally configurable, many running chains
do not change them from their default values. For example, initializing
a node on Osmosis, Terra, and the Cosmos Hub using the their `init` command produces
a `config.toml` with Tendermint's default values for these parameters.

### Why this parameter consolidation?

Reducing the number of parameters is good for UX. Fewer superfluous parameters makes
running and operating a Tendermint network less confusing. 

The Prevote and Precommit messages are both similar sizes, require similar amounts
of processing so there is no strong need for them to be configured separately.

The `TimeoutPropose` parameter governs how long Tendermint will wait for the proposed
block to be gossiped. Blocks are much larger than votes and therefore tend to be
gossiped much more slowly. It therefore makes sense to keep `TimeoutPropose` and
the `TimeoutProposeDelta` as parameters separate from the vote timeouts.

`TimeoutCommit` is used by chains to ensure that the network waits for the votes from
slower validators before proceeding to the next height. Without this timeout, the votes
from slower validators would consistently not be included in blocks and those validators
would not be counted as 'up' from the chain's perspective. Being down damages a validator's
reputation and causes potential stakers to think twice before delegating to that validator.

`TimeoutCommit` also prevents the network from producing the next height as soon as validators
on the fastest hardware with a summed voting power of +2/3 of the network's total have
completed execution of the block. Allowing the network to proceed as soon as the fastest
+2/3 completed execution would have a cumulative effect over heights, eventually
leaving slower validators unable to participate in consensus at all. `TimeoutCommit`
therefore allows networks to have greater variability in hardware. Additional
discussion of this can be found in [tendermint issue 5911][tendermint-issue-5911-comment]
and [spec issue 359][spec-359].

## Alternative Approaches

### Hardcode the parameters

Many Tendermint networks run on similar cloud-hosted infrastructure. Therefore,
they have similar bandwidth and machine resources. The timings for propagating votes
and blocks are likely to be reasonably similar across networks. As a result, the
timeout parameters are good candidates for being hardcoded. Hardcoding the timeouts
in Tendermint would mean entirely removing these parameters from any configuration
that could be altered by either an application or a node operator. Instead,
Tendermint would ship with a set of timeouts and all applications using Tendermint
would use this exact same set of values.

While Tendermint nodes often run with similar bandwidth and on similar cloud-hosted
machines, there are enough points of variability to make configuring
consensus timeouts meaningful. Namely, Tendermint network topologies are likely to be
very different from chain to chain. Additionally, applications may vary greatly in 
how long the `Commit` phase may take. Applications that perform more work during `Commit`
require a longer `TimeoutCommit` to allow the application to complete its work
and be prepared for the next height.

* From https://github.com/tendermint/spec/issues/359 it looks like `TimeoutCommit` really only exists for the purpose of allowing
precommits to be collected after the +2/3 has initially been seen. Not much more value to it than that. 

* allowing long timeout commit discussed here: https://github.com/tendermint/tendermint/issues/5911#issuecomment-973560381

## Decision

None

## Detailed Design

### New Consensus Parameters

A new `TimeoutParams` `message` will be added to the [params.proto file][consensus-params-proto].
This message will have the following form:

```proto
message TimeoutParams {
 google.protobuf.Duration timeout_propose = 1
       [(gogoproto.nullable) = false, (gogoproto.stdduration) = true];
 google.protobuf.Duration timeout_propose_delta = 2
       [(gogoproto.nullable) = false, (gogoproto.stdduration) = true];
 google.protobuf.Duration timeout_vote = 3
       [(gogoproto.nullable) = false, (gogoproto.stdduration) = true];
 google.protobuf.Duration timeout_vote_delta = 4
       [(gogoproto.nullable) = false, (gogoproto.stdduration) = true];
 google.protobuf.Duration timeout_commit = 5
       [(gogoproto.nullable) = false, (gogoproto.stdduration) = true];
 bool skip_timeout_commit = 6;
}
```

This new `message` will then be added as a field into the [`ConsensusParams`
message][consensus-params-proto]. The same default values that are [currently
set for these parameters][current-timeout-defaults] in the local configuration
file will be used as the defaults for these new consensus parameters in the
[consensus parameter defaults][default-consensus-params].

Validation of these new parameters will be identical to their [validation
currently][time-param-validation].
Namely, these parameters must be non-negative.

### Removal of Old Parameters

The old `timeout-*` parameters that are configured in the [config.toml][config-toml]
will be removed completely.

### Optional: Temporary Local Overrides

The new `TimeoutParams` will be released during the `v0.36` release cycle. Accidentally
configuring these parameters too low would result could result in chains with slow
networks or low-degree network topologies to not gossip votes within the configured
timeouts and require many rounds for consensus to occur. To prevent this condition,
we could optionally include a set of `unsafe-timeout-*` parameters in the `config.toml`.

These parameters would allow node operators to quickly remedy the situation while
preparing to update the consensus parameters. This would be a temporary solution that
we would remove within the v0.37 release.

### Add New Consensus Parameters to HashedParams

Tendermint currently only verifies that a subset of the consensus parameters are
equal across all validators. These parameters are the `BlockMaxBytes` and the `BlockMaxGas`.
A [hash of these parameters][hashed-params] is included in the block. Validators ensure 
their values of the parameters match by hashing their value of the parameters and 
checking that their hashed value matches the hash included in the block. 

Including the new parameters in the consensus parameters hash will break verification
of old blocks. We will therefore wait until other verification-breaking changes
occur and add these parameters into the hashed parameters when that occurs. Currently,
`v0.37` is planned to be a verification-breaking change, so these parameters
should be included in in the hash as part of that release. 

## Consequences

### Positive

* Timeout parameters will be equal across all of the validators in a Tendermint network.
* Remove superfluous timeout parameters.

### Negative

### Neutral

* Timeout parameters require consensus to change.

## References

[conseusus-params-proto]: https://github.com/tendermint/spec/blob/a00de7199f5558cdd6245bbbcd1d8405ccfb8129/proto/tendermint/types/params.proto#L11
[hashed-params]: https://github.com/tendermint/tendermint/blob/7cdf560173dee6773b80d1c574a06489d4c394fe/types/params.go#L49
[default-consensus-params]: https://github.com/tendermint/tendermint/blob/7cdf560173dee6773b80d1c574a06489d4c394fe/types/params.go#L79
[current-timeout-defaults]: https://github.com/tendermint/tendermint/blob/7cdf560173dee6773b80d1c574a06489d4c394fe/config/config.go#L955
[config-toml]: https://github.com/tendermint/tendermint/blob/5cc980698a3402afce76b26693ab54b8f67f038b/config/toml.go#L425-L440
[cosmos-sdk-consensus-params]: https://github.com/cosmos/cosmos-sdk/issues/6197
[time-param-validation]: https://github.com/tendermint/tendermint/blob/7cdf560173dee6773b80d1c574a06489d4c394fe/config/config.go#L1038
[tendermint-issue-5911-comment]: https://github.com/tendermint/tendermint/issues/5911#issuecomment-973560381
[spec-issue-359]: https://github.com/tendermint/spec/issues/359
[arxiv-paper]: https://arxiv.org/pdf/1807.04938.pdf
