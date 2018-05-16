# Roadmap

BREAKING CHANGES:
- Better support for injecting randomness
- Upgrade consensus for more real-time use of evidence

FEATURES:
- Use the chain as its own CA for nodes and validators
- Tooling to run multiple blockchains/apps, possibly in a single process
- State syncing (without transaction replay)
- Add authentication and rate-limitting to the RPC

IMPROVEMENTS:
- Improve subtleties around mempool caching and logic
- Consensus optimizations:
	- cache block parts for faster agreement after round changes
	- propagate block parts rarest first
- Better testing of the consensus state machine (ie. use a DSL)
- Auto compiled serialization/deserialization code instead of go-wire reflection

BUG FIXES:
- Graceful handling/recovery for apps that have non-determinism or fail to halt
- Graceful handling/recovery for violations of safety, or liveness
