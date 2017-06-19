# Tendermint Roadmap

This is an estimate of what we will be working on in Tendermint over the coming months.
It is in the same style as our [CHANGELOG](/docs/changelog)
How these changes will be rolled out in terms of versions and releases can be better [tracked on Github](https://github.com/tendermint/tendermint/issues)

Please note that Tendermint is not yet production ready;
it is pre-v1.0.0 and we make backwards incompatible changes with each minor version release.
If you require more stability in the near term, please [get in touch](/contact).

BREAKING CHANGES:

- Add more fields to the Header: NextValidatorSet, ResultsHash, EvidenceHash
- Pass evidence/voteInfo through ABCI
- Upgrade the consensus to make more real-time use of evidence during voting;
instead of +2/3 precommits for a block, a Commit becomes the entire `JSet`.  
While the commit size may grow unbounded in size, it makes a fork immediately slash a +1/3 Byzantine subset of validators.
- Avoid exposing empty blocks as a first-class citizen of the blockchain
- Use a more advanced logging system

FEATURES:

- Use the chain as its own CA for nodes and validators
- Tooling to run multiple blockchains/apps, possibly in a single process
- State syncing (without transaction replay)
- Transaction indexing and improved support for querying history and state
- Add authentication and rate-limitting to the RPC

IMPROVEMENTS:

- Better Tendermint CLI
- Improve subtleties around mempool caching and logic
- Consensus optimizations: 
	- cache block parts for faster agreement after round changes
- Better testing of the consensus state machine (ie. use a DSL)
- Auto compiled serialization/deserialization code instead of go-wire reflection

BUG FIXES:

- Graceful handling/recovery for apps that have non-determinism or fail to halt
- Graceful handling/recovery for violations of safety, or liveness
