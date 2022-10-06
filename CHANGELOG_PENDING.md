# Unreleased Changes

## v0.34.22

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- [rpc] support https inside websocket (@RiccardoM, @cmwaters)

### IMPROVEMENTS

### BUG FIXES

- [config] \#9483 Calling `tendermint init` would incorrectly leave out the new
  `[storage]` section delimiter in the generated configuration file - this has
  now been fixed
- [p2p] \#9500 prevent peers who have errored being added to the peer_set (@jmalicevic)
