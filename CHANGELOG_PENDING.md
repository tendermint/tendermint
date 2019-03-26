## v0.32.0

**

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [libs/common] Remove RepeatTimer (also TimerMaker and Ticker interface)
- [rpc/client] \#3458 Include NetworkClient interface into Client interface

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [p2p/pex] Add `p2p.seed_crawl_data_filename` config variable, which is used
  in seed mode for storing crawling data. Set to `data/seed_crawl_data.json` by
  default. If  `p2p.seed_crawl_data_filename` is empty, no data will be saved (this may lead to seed crawling a peer
  too soon if restarted; not critical).
- [rpc] \#3419 Start HTTPS server if `rpc.tls_cert_file` and `rpc.tls_key_file` are provided in the config (@guagualvcha)

### IMPROVEMENTS:

- [mempool] \#2778 No longer send txs back to peers who sent it to you

### BUG FIXES:

- [blockchain] \#2699 update the maxHeight when a peer is removed
