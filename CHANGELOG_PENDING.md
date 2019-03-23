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
- [p2p/pex] Add `p2p.seed_crawl_data_filename` config variable, which is used
  in seed mode for storing crawling data. Set to `data/seed_crawl_data.json` by
  default. If  `p2p.seed_crawl_data_filename` is empty, no data will be saved (this may lead to seed crawling a peer
  too soon if restarted; not critical).

### IMPROVEMENTS:

### BUG FIXES:

- [blockchain] \#2699 update the maxHeight when a peer is removed
