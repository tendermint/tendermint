# Tendermint

![banner](docs/tendermint-core-image.jpg)

[Byzantine-Fault Tolerant](https://en.wikipedia.org/wiki/Byzantine_fault_tolerance)
[State Machines](https://en.wikipedia.org/wiki/State_machine_replication).
Or [Blockchain](<https://en.wikipedia.org/wiki/Blockchain_(database)>), for short.

[![version](https://img.shields.io/github/tag/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/releases/latest)
[![API Reference](https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667)](https://pkg.go.dev/github.com/tendermint/tendermint)
[![Go version](https://img.shields.io/badge/go-1.15-blue.svg)](https://github.com/moovweb/gvm)
[![Discord chat](https://img.shields.io/discord/669268347736686612.svg)](https://discord.gg/vcExX9T)
[![license](https://img.shields.io/github/license/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/blob/master/LICENSE)
[![tendermint/tendermint](https://tokei.rs/b1/github/tendermint/tendermint?category=lines)](https://github.com/tendermint/tendermint)
[![Sourcegraph](https://sourcegraph.com/github.com/tendermint/tendermint/-/badge.svg)](https://sourcegraph.com/github.com/tendermint/tendermint?badge)

| Branch | Tests                                                                                      | Coverage                                                                                                                             | Linting                                                                    |
|--------|--------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| master | ![Tests](https://github.com/tendermint/tendermint/workflows/Tests/badge.svg?branch=master) | [![codecov](https://codecov.io/gh/tendermint/tendermint/branch/master/graph/badge.svg)](https://codecov.io/gh/tendermint/tendermint) | ![Lint](https://github.com/tendermint/tendermint/workflows/Lint/badge.svg) |

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state transition machine - written in any programming language -
and securely replicates it on many machines.

For protocol details, see [the specification](https://github.com/tendermint/spec).

For detailed analysis of the consensus protocol, including safety and liveness proofs,
see their recent paper, "[The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)".

I reference this paper and HotStuff, and propose a pipelined BFT consensus protocol – ChainedTendermint,
which has been published in IEEE: https://ieeexplore.ieee.org/document/9350801
I also paste it here: https://github.com/james-ray/tendermint/blob/chainedTendermint/H041.pdf
Simply state it: it parallelize the steps of Tendermint vote protocol, cancel the round in Tendermint, but keeps the votes from one replica follow the lock and unlock rule in Tendermint in the height base.
In short, it may not have much creativity in protocol design, but rather the parallel and pipleline implementations of Tendermint, and keeping its safety rule unchanged (in height base).

## Thoughts
There are three categories of consensus protocols:

1. Tendermint/HoneyBadger are the category of instant finality: the height h must be finalized before the replica can enter height h+1.
2. Chained Tendermint is of the same category of CasperFFG, Grandpa and Chained-HotStuff. This category is "not instant finalized, but can be finalized after some heights/checkpoints", and the heights/checkpoints that are needed to finalize an earlier height/checkpoint are not guaranteed.
3. Clique/Dfinity are the category of "follow the longest chain without voting", BSC and Heco use Posa, which is a modified version of Clique. Dfinity uses random beacon, improves the round robin strategy in Clique.  I also publish another paper "Continuous Distributed Key Generation on Blockchain Based on BFT Consensus", which is also based on random beacon but refreshes the distributed seed of each epoch. I don't know Dfinity uses which way to refresh the seed.

At first I don't think Chained Tendermint has much value, because category 2 and category 3 are both without instant finality, and I prefer category 3, since it does not need the broadcast of voting.   
Another reason that I think category 2 is challenging to succeed is how to guarantee the hypothesis of BFT protocol: less of 1/3 fauly voting power.
If more than 1/3 validator nodes go offline, the network is hung.
If there is two chain forks confirmed, the network might need human handle to recover, because the protocol does not consider this may happen and does not write code to handle chain forks, to follow the longer confirmed chain. 
The last reason is: there is no decentralised way to handle long range attack, all the three categories of consensus protocols have this problem. While the PoW protocol might be condemned as wasting electric, it does not need vote, it does not have hypothesis and it does not need human handle to recover, so it is robuster. A metaphor is: The PoS based public chain may be more like UN, while PoW public chain is more like DAO.

Recently lots of cross chain and L2 project succeed, such as Polygon, Polynetwork, and the eth-like chain Heco and Bsc succeed as well. I begin to think the trend is: public chains with PoW contain value, alliance chain to do the cross chain. Until recently I realize that Eth2.0 still wants to use CasperFFG and Polkadot uses Grandpa protocol and both combines the mechanism to use random numbers to do the selecting and so on. I used to think eth2.0 takes current eth as its shard 0, remains its whole data and PoW mechanism. But after reading its roadmap plan, it needs whole chain data migrate to a new shard of Eth2.0, evm changes to ewsam, and Eth2.0 needs no PoW, very much like Polkadot -- the shard does not need consensus protocol, the hub chain handles the consensus. I doubt this could succeed, as mentioned the difference like UN and DAO.

Why Heco and Bsc does not like category 1 and 2, but prefer category 3, while eth2.0 and Polkadot prefer category 2? I think the reason is: in order to do cross chain/shard, the confirmed checkpoint mechanism is mandatory, it can be confirmed later, but not none. While Heco and Bsc care more about performance, for the confirm time, since it is more like aliance chain, two heights are enough.

The design of the two--eth2.0 and Polkadot, is so similar, why they do not combine into one?

I used to contact with Alistair Steward, he explained to me the detail of Grandpa protocol, I think it is a marvelous protocol and I learned a lot from him. Besides there maybe only one downside (if there is any), it cannot adopt the aggregation of signatures scheme, because the vote set includes the votes of different heights.
And for CasperFFG, I don't think its liveness is strong. There may be situation that both competing chain forks are hard to continue, because the replica cannot violate the voting rules: s1<h1<s2<h2.   And, it needs the "direct parent" constraint, while the Chained Tendermint does not.
