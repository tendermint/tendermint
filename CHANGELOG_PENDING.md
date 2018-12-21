## v0.27.4

*TBD*

Special thanks to external contributors on this release:

### BREAKING CHANGES:

* CLI/RPC/Config
- [cli] Removed `node` `--proxy_app=dummy` option. Use `kvstore` (`persistent_kvstore`) instead.
- [cli] Renamed `node` `--proxy_app=nilapp` to `--proxy_app=noop`.
- [config] \#2992 `allow_duplicate_ip` is now set to false

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol
- multiple connections from the same IP are now disabled by default (see `allow_duplicate_ip` config option)

### FEATURES:
- [privval] \#1181 Split immutable and mutable parts of priv_validator.json

### IMPROVEMENTS:

### BUG FIXES:

