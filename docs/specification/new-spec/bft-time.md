# BFT time in Tendermint 

Tendermint provides a deterministic, Byzantine fault-tolerant, source of time.
In the context of Tendermint, time denotes UNIX time in milliseconds, i.e.,
corresponds to the number of milliseconds since January 1, 1970.

Time in Tendermint is defined with the Time field of the block header. 
It satisfies the following property:

- **Time Monotonicity**: Time is monotonically increasing, i.e., given 
a header H1 for height h1 and a header H2 for height `h2 = h1 + 1`, `H1.Time < H2.Time`.

Beyond satisfying time monotinicity, Tendermint also checks the following
property, but only when signing a prevote for a block:

- **Subjective Time Validity**: Time is greater than MinValidTime(last_block_time,
  now, round) and less than or equal to MaxValidTime(last_block_time, now), where:

```go
// wiggle and iota are provided by consensus config.
func MinValidTime(last_block_time, now time.Time, round int) time.Time {
	var minValidTime time.Time = last_block_time.Add(iota)
	if round == 0 {
		minValidTime = maxTime(minValidTime, now.Add(-1*wiggle)
	} else {
		// For all subsequent rounds, we accept any block > last_block_time+iota.
	}
	return minValidTime
}

// wiggle and wiggle_r are provided by consensus config.
func MaxValidTime(last_block_time, round int) time.Time {
	return now.
		Add(wiggle).
		Add(wiggle*wiggle_r*round)
}
```

For `MinValidTime`, we only accept recent blocks (`wiggle`) on the first
round.  This has the effect of slightly slowing down the blockchain (requiring
at least two rounds of consensus instead of one) progressively as more validator
clocks get off sync from each other. Blocks that are significantly older than
`now` can still be valid (except in round 0), which allows for the re-proposing of older proposals.
The blockchain's time "eventually" catches up over block heights to a reasonably recent time as long as
time-correct validators' proposals are committed in a timely fashion (i.e. less than 1/3
are Byzantine)  TODO: Quantify "eventually" as a function of % of time-correct
validators.

For `MaxValidTime`, we accept blocks where the block time is greater than `now`
plus some threshold that increases linearly with the round number. 
The purpose of `wiggle_r` is for graceful degredation when +2/3 validators
*aren't* within `wiggle` of each other but are otherwise non-Byzantine in all other respects. 
Consider an example with 100 equally weighted validators, where 33 are Byzantine,
and one of the remaining 67 validators has a faulty clock that causes it to drift
back more than `wiggle` from the other 66.  If the 33 Byzantine
validators were to withhold their votes, no block would produce a Polka until the
drifting one becomes the proposer!

NOTE: `wiggle_r` could be set to something like 0.05 (e.g. if `wiggle` were 20s, `wiggle*wiggle_r` would be 1s.)
`wiggle*wiggle_r` should probably be less than `timeout_propose_delta` to prevent unnecessary
forward time-jumps in cases where higher rounds are reached due to factors
other than network performance -- for example, where the network is performant
but several proposers were absent in a row.  `wiggle_r` could theoretically be set to 0
if it can be assumed that +2/3 (by voting power) of correct validators' clocks are within `wiggle` of each other.

Subjective time validity is ignored when a Polka or Commit is found, allowing
consensus to progress locally even when the subjective time requirements are not satisfied.
