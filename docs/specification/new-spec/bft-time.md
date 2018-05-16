# BFT time in Tendermint 

Tendermint provides a deterministic, Byzantine fault-tolerant, source of time. 
Time in Tendermint is defined with the Time field of the block header. 

It satisfies the following property:

- Time Monotonicity: Time is monotonically increasing, i.e., given 
a header H1 for height h1 and a header H2 for height `h2 = h1 + 1`, `H1.Time < H2.Time`. 

Beyond satisfying time monotinicity, Tendermint also checks the following
propoerty only just before signing a prevote:

- Temporal Time Validity: Time is greater than MinValidTime(last_block_time,
  new_block_time, now, ideal_block_duration, round) and MaxValidTime(last_block_time, now, round)

```go
func MinValidTime(last_block_time, new_block_time, now time.Time, ideal_block_duration time.Duration, round int) error {
	if !last_block_time.Before(new_block_time) {
		return errors.New("New block time cannot be before the last block time")
	}
	if !new_block_time.Before(now) {
		return errors.New("New block time cannot be in the future")
	}
}
```

In the context of Tendermint, time denotes UNIX time in milliseconds, i.e.,
corresponds to the number of milliseconds since January 1, 1970.




the time monotinicity only occurs at the prevote step.  When 
Let's consider the following example:
 - we have four processes p1, p2, p3 and p4, with the following voting power distribution (p1, 23), (p2, 27), (p3, 10)
and (p4, 10). The total voting power is 70 (`N = 3f+1`, where `N` is the total voting power, and `f` is the maximum voting 
power of the faulty processes), so we assume that the faulty processes have at most 23 of voting power. 
Furthermore, we have the following vote messages in some LastCommit field (we ignore all fields except Time field): 
      - (p1, 100), (p2, 98), (p3, 1000), (p4, 500). We assume that p3 and p4 are faulty processes. Let's assume that the 
      `block.LastCommit` message contains votes of processes p2, p3 and p4. Median is then chosen the following way: 
      the value 98 is counted 27 times, the value 1000 is counted 10 times and the value 500 is counted also 10 times. 
      So the median value will be the value 98. No matter what set of messages with at least `2f+1` voting power we 
      choose, the median value will always be between the values sent by correct processes.   

We ensure Time Monotonicity and Time Validity properties by the following rules: 
  
- let rs denotes `RoundState` (consensus internal state) of some process. Then 
`rs.ProposalBlock.Header.Time == median(rs.LastCommit) &&
rs.Proposal.Timestamp == rs.ProposalBlock.Header.Time`.

- Furthermore, when creating the `vote` message, the following rules for determining `vote.Time` field should hold: 

    - if `rs.Proposal` is defined then 
    `vote.Time = max(rs.Proposal.Timestamp + 1, time.Now().Round(0).UTC())`, where `time.Now().Round(0).UTC()` 
    denotes local Unix time in milliseconds.  
    
    - if `rs.Proposal` is not defined and `rs.Votes` contains +2/3 of the corresponding vote messages (votes for the 
    current height and round, and with the corresponding type (`Prevote` or `Precommit`)), then 
    
    `vote.Time = max(median(getVotes(rs.Votes, vote.Height, vote.Round, vote.Type)), time.Now().Round(0).UTC())`,
    
    where `getVotes` function returns the votes for particular `Height`, `Round` and `Type`.  
    The second rule is relevant for the case when a process jumps to a higher round upon receiving +2/3 votes for a higher 
    round, but the corresponding `Proposal` message for the higher round hasn't been received yet.    


