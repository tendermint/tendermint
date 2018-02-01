# BFT time in Tendermint 

Tendermint provides a deterministic, Byzantine fault-tolerant, source of time. 
Time in Tendermint is defined with the Time field of the block header. 

It satisfies the following properties:

- Time Monotonicity: Time is monotonically  increasing, i.e., given 
a header H1 for height h1 and a header H2 for height `h2 = h1 + 1`, `H1.Time < H2.Time`. 
- Time Validity: Given a set of Commit votes that forms the `block.LastCommit` field, a range of 
valid values for the Time field of the block header is defined only by  
Precommit messages (from the LastCommit field) sent by correct processes, i.e., 
a faulty process cannot arbitrarily increase the Time value.  

In the context of Tendermint, time is of type int64 and denotes UNIX time in milliseconds, i.e., 
corresponds to the number of milliseconds since January 1, 1970. Before defining rules that need to be enforced by the 
Tendermint consensus protocol, so the properties above holds, we introduce the following definition:

- median of a set of `Vote` messages is equal to the median of `Vote.Time` fields of the corresponding `Vote` messages

We ensure Time Monotonicity and Time Validity properties by the following rules: 
  
- let rs denotes `RoundState` (consensus internal state) of some process. Then 
`rs.ProposalBlock.Header.Time == median(rs.LastCommit) &&
rs.Proposal.Timestamp == rs.ProposalBlock.Header.Time`.

- Furthermore, when creating the `vote` message, the following rules for determining `vote.Time` field should hold: 

    - if `rs.Proposal` is defined then 
    `vote.Time = max(rs.Proposal.Timestamp + 1, time.Now())`, where `time.Now()` 
    denotes local Unix time in milliseconds.  
    
    - if `rs.Proposal` is not defined and `rs.Votes` contains +2/3 of the corresponding vote messages (votes for the 
    current height and round, and with the corresponding type (`Prevote` or `Precommit`)), then 
    
    `vote.Time = max(median(getVotes(rs.Votes, vote.Height, vote.Round, vote.Type)), time.Now())`,
    
    where `getVotes` function returns the votes for particular `Height`, `Round` and `Type`.  
    The second rule is relevant for the case when a process jumps to a higher round upon receiving +2/3 votes for a higher 
    round, but the corresponding `Proposal` message for the higher round hasn't been received yet.    


