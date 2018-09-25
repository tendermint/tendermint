# Pending

Special thanks to external contributors on this release:

BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [node] Remove node.RunForever

FEATURES:

IMPROVEMENTS:
- [consensus] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics
- [p2p] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics

BUG FIXES:
- [autofile] \#2428 Group.RotateFile need call Flush() before rename (@goolAdapter)
- [node] \#2434 Make node respond to signal interrupts while sleeping for genesis time
