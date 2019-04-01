## v0.31.2

*March 30th, 2019*

This release fixes a regression from v0.31.1 where Tendermint panics under
mempool load for external ABCI apps.

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
- [libs/autofile] \#3504 Remove unused code in autofile package. Deleted functions: `Group.Search`, `Group.FindLast`, `GroupReader.ReadLine`, `GroupReader.PushLine`, `MakeSimpleSearchFunc` (@guagualvcha)

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [p2p/pex] Add `p2p.seed_crawl_data_filename` config variable, which is used
  in seed mode for storing crawling data. Set to `data/seed_crawl_data.json` by
  default. If  `p2p.seed_crawl_data_filename` is empty, no data will be saved (this may lead to seed crawling a peer
  too soon if restarted; not critical).

### IMPROVEMENTS:

- [circle] \#3497 Move release management to CircleCI

### BUG FIXES:

- [mempool] \#3512 Fix panic from concurrent access to txsMap, a regression for external ABCI apps introduced in v0.31.1
