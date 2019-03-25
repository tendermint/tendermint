## v0.32.0

**

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [libs/common] Remove RepeatTimer (also TimerMaker and Ticker interface)

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [rpc] \#3419 Start HTTPS server if `rpc.tls_cert_file` and `rpc.tls_key_file` are provided in the config (@guagualvcha)

### IMPROVEMENTS:

- [mempool] \#2778 No longer send txs back to peers who sent it to you

### BUG FIXES:

- [blockchain] \#2699 update the maxHeight when a peer is removed
