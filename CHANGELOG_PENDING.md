## v0.31.2

**

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [rpc] \#3534 Add support for batched requests/responses in JSON RPC
- [p2p] \#3463 Do not log "Can't add peer's address to addrbook" error for a private peer

### BUG FIXES:
- [p2p] \#2716 Check if we're already connected to peer right before dialing it (@melekes)
- [docs] \#3514 Fix block.Header.Time description (@melekes)
