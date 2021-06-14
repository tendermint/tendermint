# ADR 069: Flexible Node Initialization

## Changlog

- 2021-06-09: Initial Draft (@tychoish)

## Context

In an effort to support [Go-API-Stability](./adr-060-go-api-stability.Md),
during the 0.35 development cycle, we have attempted to reduce the total size
of the API surface area by moving most of the interface of the `node` package
into unexported functions, as well as moving the reactors to an `internal`
package. Having this coincide with the 0.35 release made a lot of sense
because these interfaces were _already_ changing as a result of the `p2p`
[refactor](./adr-061-p2p-refactor-scope.md), so it made sense to think a bit
more about how tendermint exposes this API.

It is not immediately intuitive that the nexus of this change is in the
relationship between the `rpc` package and the `node` package, particularly
with regards to how the node instances are started. However, this is indeed
the case and the existing dependencies are tangled around supporting these
cases.

## Observations

To prepare for this work I've been playing in [this
branch](https://github.com/tendermint/tendermint/tree/tychoish/scratch-node-minimize),
with the node implementation to see if, without making major
modifications or changing interfaces, what aspects could be
simplified. My conclusions, generally: 

- The implementation of the `node` service, doesn't really need the
  ability to introspect the implementations of reactors in most
  cases.

- The exception to this, is the cluster of
  consensus/statesync/blockhain (which depend on eachother,) because
  `node` itself contains some of their specific initialization logic.

- While the RPC environment construction happens in `node`, this can
  happen early during `node` construction rather than at
  initialization time, which reduces the need for the implementation
  of the node object to track this state.

Ideally, it seems like a node instance should be expressable simply as
a list of `service.Services` and any knowledge of the internals of any
of these services could be limited to their constructors. I sketched
out a generic and simple `groupService` implementation, to handle a
desecrate stage or sequence of services.  As some services have
dependencies during construction or initialization and as a starting
point, a cluster of dependencies can be wrapped as a single
`service.Service`.

## Goals

- Provide a more flexible internal framework for initializing tendermint
  nodes to make the initatilization process less hard-coded by the
  implementation of the node objects. 
  
  - Reactors should not need to expose their interfaces *within* the
    implementation of the node type, except in the context of some groups of
    service. 

- Expose some "unsafe"  way of replacing (some?) existing components, and or
  inserting arbitrary reactors or services into the tendermint node's
  initialization.
  
  - This may include creating an exported, top-level "unsafe" package with
    several interfaces and constructors, or simply a function in the
    node package. 

- Refactor and simplify the process start/initialization logic,
  separately from--but as a precursor to--permitting user replacement
  of components.

## Non-Goals

- Fully abstract dependency-based process initialization. 

- Fully abstract replacement of core tendermint components without vendoring.

- Parallelized initialization of components.

## Questions

- To what extent does this new initialization framework need to accommodate
  the legacy p2p stack? Would it be possible to delay a great deal of this
  work to the 0.36 cycle to avoid this complexity? 
  
- There's clear and common interest in injecting some kind of mempool, and
  intuitively this makes sense as the highest priority.
  
  - Does the existing `mempool.Mempool` interface make sense to expose and
    commit to, or should we make a smaller interface? 
    
  - Is it reasonable for users to bring their own mempool without bringing
    their own reactor?

- Are there reactors, other than the mempool, that make sense to support
  ad-hoc replacing (e.g. statesync, blockchain (fastsync),) and if so should
  we provide any first-class support for this?

- There's a dependency (and nearly a cycle), between consensus,
  blockchain (fastsync), which makes it very hard to allow meaningful
  injection of any of these components without allowing injection of
  all of them (statesync provider as a
  possible exception.) Is this acceptable? 

## Future Work

- Improve or simplify the `service.Service` interface. There are some
  pretty clear limitations with this interface as written (there's no
  way to timeout slow startup or shut down, the cycle between the
  `service.BaseService` and `service.Service` implementations is
  troubling, the default panic in `OnReset` seems troubling.)

- Support explicit dependencies between components and allow for
  parallel startup, so that different reactors can startup at the same
  time, where possible.
