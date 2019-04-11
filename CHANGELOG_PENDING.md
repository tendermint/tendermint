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

- [state] [\#3438](https://github.com/tendermint/tendermint/pull/3438) 
  Persist validators every 100000 blocks even if no changes to the set
  occurred (@guagualvcha). This
  1) Prevents possible DoS attack using `/validators` or `/status` RPC
  endpoints. Before response time was growing linearly with height if no
  changes were made to validator set.
  2) Fixes performance degradation in `ExecCommitBlock` where we call
  `LoadValidators` for each `Evidence` in the block.
- [p2p] \#2716 Check if we're already connected to peer right before dialing it (@melekes)
- [docs] \#3514 Fix block.Header.Time description (@melekes)
