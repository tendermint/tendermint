# BFT time in Tendermint 

Tendermint provides a deterministic, Byzantine fault-tolerant, source of time.
In the context of Tendermint, time denotes UNIX time in milliseconds, i.e.,
corresponds to the number of milliseconds since January 1, 1970.

Time in Tendermint is defined with the Time field of the block header. 
It satisfies the following property:

- Time Monotonicity: Time is monotonically increasing, i.e., given 
a header H1 for height h1 and a header H2 for height `h2 = h1 + 1`, `H1.Time < H2.Time`. 

Beyond satisfying time monotinicity, Tendermint also checks the following
property, but only when signing a prevote for a block:

- Temporal Time Validity: Time is greater than MinValidTime(last_block_time,
  now, round) and less than or equal to MaxValidTime(now, round), where:

```go
func MinValidTime(last_block_time, now time.Time, round int) time.Time {
	var minValidTime time.Time = last_block_time.Add(iota) // iota provided by consensus params
	if round == 0 {
		minValidTime = maxTime(minValidTime, now.Subtract(wiggle) // wiggle provided by consensus params
	} else {
		// For all subsequent rounds, we accept any block > last_block_time+iota.
		return minValidTime
	}
}

func MaxValidTime(now time.Time, round int) time.Time {
	return now.Add(wiggle * (round+1))
}
```

For MinValidTime, we only accept recent blocks (i.e. "wiggle") on the first
round.  This has the effect of slowing down the blockchain progressively for 1
round, as more validator clocks go off sync.  Otherwise, the only remaining
restriction is that each block time must increment by iota.

For MaxValidTime, we accept blocks where the block time is greater than now, but
linearly with the round number.
