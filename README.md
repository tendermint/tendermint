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

I reference this paper and HotStuff, and propose a pipelined BFT consensus protocol – chainedTendermint,
which has been published in IEEE: https://ieeexplore.ieee.org/document/9350801
I also paste it here: https://github.com/james-ray/tendermint/blob/chainedTendermint/H041.pdf

## Releases

I want to implement the pipleline protocol myself in my spare time. 
If you are interested, have any question, or find any flaw in this paper, please contact me JamesRayLei@gmail.com

I plan to work on the branch chainedTendermint of this repository, any cooperative work is appreciated.


## Challenges and Plans

The pipeline protocol relies on aggregated signature, which is not done in Tendermint yet.
There are some discussion about the timestamp, each vote has different timestamp, so we need to address this problem.

Anyway, I plan to implenment the first phase protocol: the basic parallel Tendermint protocol, which does not adopt aggregated signature, but only
parallelize the prevotes and precommits, and "flatten" the rounds.  I think this needs lots of code modification.

If this protocol has been done and well tested, I will call it release 1. When refer to test, I mean, to deploy several nodes as a cluster and run the new consensus protocol. Of course, all the CI tests are unable to run, because there is no concept of "round" in parallel Tendermint protocol, the modification is big, lots of data structures would change. I will first test the cluster on normal condition, and on some abnormal network condition to see if the consensus protocol works well. The CI tests shall be fixed after the main codes pass the tests and are stable.

## Thoughts
From my point of view, BFT consensus cannot be appied to public blockchain, but only suitable for alliance chain. Though alliance chain can be used to do cross public chain txs, like Cosmos does. The difficulty is, you use BFT, so obviously you don't trust every node in the cluster, but the PoS based scheme and BFT premise can only guarantee the protocol runs well under at most 1/3 malicious nodes. Though it is not a public chain, but only an alliance chain, this condition is still hard to be guaranteed. There are two methods, one is using CA node, like the permissioned alliance chain. The other is controlling most of the funds in the operator itself, so other funds must be less than 1/3.  Whether you use which method, it is like you deployed most cluster nodes by yourself, and the other joined nodes are more like a mere formality without pratical meaning. If you can trust every node or you deploy all the nodes, you can simply use Clique consensus, which is much light weighted. 
One advantage of BFT consenus like Tendermint is instant finality, but it is based on the 1/3 malicious nodes hypothesis. If this hypothesis is broken, there are possibly two forks that meet the commit condition, and there are no rollback and follow the longest chain mechanism.
Unlike the concern of Layer2 scheme like optimistic rollup, the validators in Layer1 cannot steal funds, the most malicious thing they can do is to fork the chain. In current Tendermint, if the chain is forked then the whole chain is possibly stopped (1/2 nodes confirm one fork and the other 1/2 confirm the other fork), in other word, the attacker cannot make a double spending either. This lack of motivation increases the safety of the BFT scheme in Layer1.
In conclusion, I think using BFT consensus is conducive to let others join and constrain their behavior, if they do malicious action, their staked funds are slashed, this is an effective deterrent. But this is based on the most funds are staked in the trusted nodes, and the trusted nodes do not do malicious action.  This is like the second method that I mentioned above. If you cannot control the flow of the most funds in your alliance chain, and the funds in your chain are not achieved by paying the real world cash (in other word, the evil joined nodes accquire the funds at no cost, the safety of your chain is vulnerable.

