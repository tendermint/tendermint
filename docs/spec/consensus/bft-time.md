# BFT Time

Tendermint provides a deterministic, Byzantine fault-tolerant, source of time. 
Time in Tendermint is defined with the Time field of the block header. 

It satisfies the following properties:

- Time Monotonicity: Time is monotonically increasing, i.e., given 
a header H1 for height h1 and a header H2 for height `h2 = h1 + 1`, `H1.Time < H2.Time`. 
- Time Validity: Given a set of Commit votes that forms the `block.LastCommit` field, a range of 
valid values for the Time field of the block header is defined only by  
Precommit messages (from the LastCommit field) sent by correct processes, i.e., 
a faulty process cannot arbitrarily increase the Time value.  

In the context of Tendermint, time is of type int64 and denotes UNIX time in milliseconds, i.e., 
corresponds to the number of milliseconds since January 1, 1970. Before defining rules that need to be enforced by the 
Tendermint consensus protocol, so the properties above holds, we introduce the following definition:

- median of a Commit is equal to the median of `Vote.Time` fields of the `Vote` messages,
where the value of `Vote.Time` is counted number of times proportional to the process voting power. As in Tendermint
the voting power is not uniform (one process one vote), a vote message is actually an aggregator of the same votes whose 
number is equal to the voting power of the process that has casted the corresponding votes message.

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

    - if `rs.LockedBlock` is defined then
    `vote.Time = max(rs.LockedBlock.Timestamp + config.BlockTimeIota, time.Now())`, where `time.Now()` 
        denotes local Unix time in milliseconds, and `config.BlockTimeIota` is a configuration parameter that corresponds
        to the minimum timestamp increment of the next block.
        
    - else if `rs.Proposal` is defined then 
    `vote.Time = max(rs.Proposal.Timestamp + config.BlockTimeIota, time.Now())`,
    
    - otherwise, `vote.Time = time.Now())`. In this case vote is for `nil` so it is not taken into account for 
    the timestamp of the next block. 


