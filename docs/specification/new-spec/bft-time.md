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

- **Subjective Time Validity**: Time is greater than MinValidTime(last_block_time) and less than or equal to MaxValidTime(now, round), where:

```go
// iota provided by consensus config.
func MinValidTime(last_block_time) time.Time {
	return lastBlockTime.
		Add(time.Duration(iota) * time.Millisecond)
}

// wiggle and wiggle_delta are provided by consensus config.
func MaxValidTime(now time.Time, round int) time.Time {
	return now.
		Add(time.Duration(wiggle) * time.Millisecond).
		Add(
			time.Duration(
				wiggle_delta*float64(round),
			) * time.Millisecond,
		)
}
```

For `MinValidTime`, we accept any block that is at least `iota` greater than
the last block time. Blocks that are significantly older than the current time
can still be valid, which allows for the re-proposing of older proposals.

For `MaxValidTime`, we accept block times greater than `now` plus some
threshold that increases linearly with the round number.  The purpose of
`wiggle_delta` is for graceful degredation when +2/3 validators *aren't* within
`wiggle` of each other but are otherwise non-Byzantine in all other respects. 

Consider an example with 100 equally weighted validators, where 33 are
Byzantine, and one of the remaining 67 validators has a faulty clock that
causes it to drift back more than `wiggle` from the other 66.  Without
`wiggle_delta`, if the 33 Byzantine validators were to withhold their votes, no
block would produce a Polka until the drifting one becomes the proposer.

NOTE:  `wiggle_delta` should probably be less than `timeout_propose_delta` to
prevent unnecessary forward time-jumps in cases where higher rounds are reached
due to factors other than network performance -- for example, where the network
is performant but several proposers were absent in a row.  `wiggle_delta` could
theoretically be set to 0 if it can be assumed that +2/3 (by voting power) of
correct validators' clocks are within `wiggle` of each other.

Subjective time validity is ignored when a Polka or Commit is found, allowing
consensus to progress locally even when the subjective time requirements are
not satisfied.
