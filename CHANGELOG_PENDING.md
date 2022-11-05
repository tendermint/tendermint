# Unreleased Changes

## v0.34.23

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS
- [p2p] \#9641 Add new Envelope type and associated methods for sending and receiving Envelopes instead of raw bytes.
  This also adds new metrics, `tendermint_p2p_message_send_bytes_total` and `tendermint_p2p_message_receive_bytes_total`, that expose how many bytes of each message type have been sent.
- [rpc] \#9666 Enable caching of RPC responses (@JayT106)

### BUG FIXES
