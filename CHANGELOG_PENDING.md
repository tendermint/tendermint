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

### BUG FIXES:

- [state] Persist validators every 100000 blocks even if no changes to the set
  occurred. This prevents possible DoS attack using /validators RPC endpoint.
  Before /validators response time was growing linearly if no changes were made
  to validator set. (@guagualvcha)
- [p2p] \#2716 Check if we're already connected to peer right before dialing it (@melekes)
- [docs] \#3514 Fix block.Header.Time description (@melekes)
