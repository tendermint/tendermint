## v0.27.1

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [config] `allow_duplicate_ip` is now set to false

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol
- multiple connections from the same IP are now disabled by default (see `allow_duplicate_ip` config option)

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
- [kv indexer] \#2912 don't ignore key when executing CONTAINS
- [config] \#2980 fix cors options formatting
