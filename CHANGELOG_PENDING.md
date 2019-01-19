## v0.29.0

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [node] \#3082 MetricsProvider now requires you to pass a chain ID

* Blockchain Protocol
  * [merkle] \#2713 Merkle trees now match the RFC 6962 specification
  * [types] \#3078 Re-order Timestamp and BlockID in CanonicalVote so it's
    consistent with CanonicalProposal (BlockID comes
    first)
  * [types] \#3165 Hash of ConsensusParams only includes BlockSize.MaxBytes and
    BlockSize.MaxGas

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [rpc] \#3065 Return maxPerPage (100), not defaultPerPage (30) if `per_page` is greater than the max 100.
- [instrumentation] \#3082 Add `chain_id` label for all metrics

### BUG FIXES:
- [log] \#3060 Fix year format
- [crypto] \#3164 Update `btcd` fork for rare signRFC6979 bug
