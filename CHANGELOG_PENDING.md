## v0.29.0

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [types] consistent field order of `CanonicalVote` and `CanonicalProposal`

* Apps

* Go API
- [node] \#3082 MetricsProvider now requires you to pass a chain ID

* Blockchain Protocol
  * [merkle] \#2713 Merkle trees now match the RFC 6962 specification
  * [consensus] \#2960 normalize priorities to not exceed 2*TotalVotingPower to mitigate unfair proposer selection 
  heavily preferring earlier joined validators in the case of an early bonded large validator unbonding

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [rpc] \#3065 return maxPerPage (100), not defaultPerPage (30) if `per_page` is greater than the max 100.
- [instrumentation] \#3082 add 'chain_id' label for all metrics

### BUG FIXES:
- [log] \#3060 fix year format
