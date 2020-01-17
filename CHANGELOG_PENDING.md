## v0.33.1

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

  - [rpc/net] `listeners` is now `listen_address` with the actual bound listen address.

- Apps

- Go API

  - [node] Replace `Listeners() []string` from `Node` with `ListenAddress() string` - furthermore `ListenAddress` is 
  now read from the transport's listener so will be accurate when port is dynamically allocated as in 
  `P2P.ListenAddress = "tcp://localhost:0"

- Blockchain Protocol

- P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:


