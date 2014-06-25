## Channel ""

Filter: None

Messages:
* RefreshFilterMsg
* PeerExchangeMsg


## Channel "block"

Filter: Custom filter.

Messages:
* BlockMsg
* HeaderMsg


## Channel "mempool"

Filter: Bloom filter (n:10k, p:0.02 -> k:6, m:10KB)

FilterRefresh: Every new block.

Messages:
* MempoolTxMsg


## Channel "consensus"                                                                                                                                                                                                                                                                                                                                                    

Filter: Bitarray filter

FilterRefresh: Every new block.

Messages:
* ProposalMsg
* VoteMsg
* NewBlockMsg
