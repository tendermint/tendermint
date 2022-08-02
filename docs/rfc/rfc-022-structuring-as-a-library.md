# RFC 022: Structuring Tendermint as a Library

## Changelog

- 25 July 2022: Initial draft (@cmwaters)

## Abstract

How do we want our users to use Tendermint? How does Tendermint best deliver it's unique value proposition in a way that moves it towards being the leading generalizable BFT consensus engine? This document aims to explore these themes mainly through comparison of Tendermint as a library or Tendermint as a service. The RFC focuses just on product direction and offers no concrete technical solutions although decisions here will dictate future technical discussions as we look to improve the ergonomics of the repository.

## Background

The word "library" is not used once in the codebases' documentation, however, it is understood that this was perhaps the intention of the original authors. As the codebase expanded and adoption grew, there became a need to distinguish between private and public APIs to offer some stability whilst still developing the software. This culminated in [ADR 060](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-060-go-api-stability.md) which internalized the majority of functionality behind an `internal` directory. This reflected the thinking at the time which endeavoured to present Tendermint as a simple service that one could run without having to worry about what was really happening. For those that had business concerns that required more than what was offered, they had to fork the codebase as is evident with [Celestia](https://github.com/celestiaorg/celestia-core). While there have been numerous discussions over the past years, nothing has been explicitly documented as this RFC aims to do.

## Discussion

In simpler terms, there are two models Tendermint can look to follow: A library or a service.

### Definitions

Given that these two words: library and service are already overloaded as it is, I want to quickly draw out what these paths actually entail. When talking about a library, we want to structure Tendermint to be composable. Users should be able to take the parts they want and replace or build out the parts that are new and easily fit them together. When talking about a service, we're talking about a simple, straight out of the box, batteries-included consensus as a service: Tendermint takes in bytes and ensures that multiple machines replicate it withstranding a degree of arbitrary failures.

### Tradeoffs

When offering Tendermint as a service:

- We're striving to abstract away the complexity and expose a simple surface for users to build on top of.
- We minimize configurability by trying to make reasonable estimations about how the software is used so one can simply "plug and play".

As a service:

- We have a small surface area to maintain and can hide messy internals. It makes it easier to change internal components and offer strong API stability guarantees.
- We're happy with forks. We aim to appease 80% of the users and support forking as a strategy for custom Tendermint implementations.

When offering Tendermint as a library:

- We're focusing on customizability, extensability and composability.
- We want our users to take more time to understand how Tendermint works

As a library:

- We have to be wary of cross-component security concerns. We need to be very explicit about the invariants within each module.
- Tendermint has already been broken down into separate modules based on functionality so there already exists a good starting point with which to build into a library.
- We are committing to more documentation/tutorials and general support for users of the library.
- Tendermint is better aligned with the Cosmos DNA which favours notions of soverignty and the ability to make whatever application specific blockchain you can think of.
- We reduce the likelihood of forking but understand the risk that failure to correctly abstract out components may frustrate users.
- We think modules is a good way to encourage others to innovate that may upstream into mainline Tendermint.

While presenting these as two options, it's possible that we opt to do both but we must understand the extra work that this entails.

