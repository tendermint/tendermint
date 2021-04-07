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
If you are intrested, have any question, or find any flaw in this paper, please contact me JamesRayLei@gmail.com

I plan to work on the branch chainedTendermint of this repository, any cooperative work is appreciated.


## Challenges

The pipeline protocol relies on aggregated signature, which is not done in Tendermint yet.
There are some discussion about the timestamp, each vote has different timestamp, so we need to address this problem.

Anyway, I plan to implenment the first phase protocol: the basic parallel Tendermint protocol, which does not adopt aggregated signature, but only
parallelize the prevotes and precommits, and "flatten" the rounds.  I think this needs lots of code modification.

If this protocol has been done and well tested, I will call it release 1. When refer to test, I mean, to deploy several nodes as a cluster and run the new consensus protocol. Of course, all the CI tests are unable to run, because there is no concept of "round" in parallel Tendermint protocol, the modification is big, lots of data structures would change. I will first test the cluster on normal condition, and on some abnormal network condition to see if the consensus protocol works well. The CI tests shall be fixed after the main codes pass the tests and are stable.
