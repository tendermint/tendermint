# ADR 069: Flexible Node Initialization

## Changlog

- 2021-06-09: Initial Draft (@tychoish)

## Status 

Proposed. 

## Context

In an effort to support [Go-API-Stability](./adr-060-go-api-stability.Md),
during the 0.35 development cycle, we have attempted to reduce the total size
of the API surface area by moving most of the interface of the `node` package
into unexported functions, as well as moving the reactors to an `internal`
package. Having this coincide with the 0.35 release made a lot of sense
because these interfaces were _already_ changing as a result of the `p2p`
[refactor](./adr-061-p2p-refactor-scope.md), so it made sense to think a bit
more about how tendermint exposes this API.

While the interfaces of the P2P layer and most of the node package are
already internalized, this precludes several kinds of operational
patterns that are important to key tendermint users who use tendermint
as a library, specifically introspecting the tendermint node service
and replacing components is not supported in the latest version of the
code, and some of these use cases would require maintaining a vendor
copy of the code. Adding these features requires rather extensive
(internal/implementation) changes to the `node` and `rpc` packages,
and this ADR describes a model for changing the way that tendermint
nodes initialize, in service of providing this kind of functionality.

We consider node initialization, because the current implemention
provides strong connections between all components, as well as between
the components of the node and the RPC layer, and being able to think
about the interactions of these components will help enable these
features and help define the requirements of the node package. 

## Alternative Approaches

### Do Nothing

The current implementation is functional and sufficient for the vast
majority of use cases (e.g. all users of the Cosmos-SDK as well as
anyone who runs tendermint and the ABCI application in separate
processes.) In the current implementation, and even previous versions,
modifying node initialization or injecting custom components required
copying most of the `node` package, which amounts to requiring users
to maintain a vendored copy of tendermint for these kinds of use
cases.

While this is (likely) not tenable in the long term, as users do want
more modularity, and the current service implementation is brittle and
difficult to maintain, it may be viable in the short and medium term
to delay implementation which will allow us to do more product
research and build more confidence and consensus around the eventual
solution.

### Generic Service Plugability

We can imagine a system design that exports interfaces (in the Golang
sense) for all components of the system, to permit runtime dependency
injection of all components in the system so that users can compose
tendermint nodes of arbitrary user supplied components. 

This is an interesting proposal, and may be interesting to persue, but
essentially requires doing a lot more API design and increases the
surface area of our API. While this is not a goal at the moment,
eventually providing support for some kinds of plugability may be
useful, so the current solution does not explicitly forclose the
possibility of this alternative. 

### Abstract Dependency Based Startup and Shutdown

The proposal on in this document simplifies and makes tendermint node
initialization more abstract, but the system lacks a number of
features which daemon/service initialization might provide, such as a
dependency based system that allows the authors of services to control
the initialization and shutdown order of components using an
dependencies to control the ordering. 

Dependency based orderings make it possible to write components
(reactors, etc.) with only limited awareness of other components or of
node initialization, and is the state of the art for process
initialization. However, this may be too abstract and complicated, and
authors of components in the current implementation of tendermint
*would* need to know about other components, so a dependency based
system would be unhelpfully abstract at this stage.

## Decisions

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

## Detailed Design

## Consequences 

### Positive

### Negative

### Neutral 

## Open Questions

### Timing

### Interface Implications

## References 

------------------------------------------------------------------------

## Observations

To prepare for this work [this
branch](https://github.com/tendermint/tendermint/tree/tychoish/scratch-node-minimize)
contains some experimentation with the node implementation to see if,
without making major modifications or changing interfaces, what
aspects could be simplified. To summarize, generally:

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
of these services could be limited to their constructors. There is a
sketch of a generic and simple `groupService` implementation, to handle a
desecrate stage or sequence of services.  As some services have
dependencies during construction or initialization and as a starting
point, a cluster of dependencies can be wrapped as a single
`service.Service`.

## Questions

- To what extent does this new initialization framework need to accommodate
  the legacy p2p stack? Would it be possible to delay a great deal of this
  work to the 0.36 cycle to avoid this complexity? 
  
  - Answer: _depends on timing_, and the requirement to ship pluggable
    reactors in 0.35. 
  
- There's clear and common interest in injecting some kind of mempool, and
  intuitively this makes sense as the highest priority.
  
  - Does the existing `mempool.Mempool` interface make sense to expose and
    commit to, or should we make a smaller interface? 
    
	- Answer: _yes_, smaller mempool interface is better. 

  - Is it reasonable for users to bring their own mempool without bringing
    their own reactor?
	
	- Answer: _probably not_, but this might be a function of how the
      current implementations work and can be refactored. 

- Are there reactors, other than the mempool, that make sense to support
  ad-hoc replacing (e.g. statesync, blockchain (fastsync),) and if so should
  we provide any first-class support for this?

  - Answer: _defer_. Certainly this shouldn't be attempted until after
    there is a refactor of the consensus reactor. Is more likely than
    block/fast sync. 

- There's a dependency (and nearly a cycle), between consensus,
  blockchain (fastsync), which makes it very hard to allow meaningful
  injection of any of these components without allowing injection of
  all of them (statesync provider as a possible exception.) Is this
  acceptable?
  
  - Answer: _yes_. 

## Future Work

- Improve or simplify the `service.Service` interface. There are some
  pretty clear limitations with this interface as written (there's no
  way to timeout slow startup or shut down, the cycle between the
  `service.BaseService` and `service.Service` implementations is
  troubling, the default panic in `OnReset` seems troubling.)

- As part of the refactor of `service.Service` have all services/nodes
  respect the lifetime of a `context.Context` object, and avoid the
  current practice of creating `context.Context` objects in p2p and
  reactor code. This would be required for in-process multi-tenancy.
  
- Consider additional interfaces that node objects can provide in
  terms of access to components (e.g. the mempool).

- Support explicit dependencies between components and allow for
  parallel startup, so that different reactors can startup at the same
  time, where possible.

## Proposal / Conclusion 

- Wait for e2e tests to stablize, before changing node initialization.

- Continue building on [the experimental branch
  branch](https://github.com/tendermint/tendermint/tree/tychoish/scratch-node-minimize)
  to reduce the complexity in the node initialization, and recast
  `nodeImpl` as a list of `service.Service` objects, modulo
  requirements for legacy p2p components which will be deleted.

- Add the ability to construct a tendermint node via a new public
  constructor that will take a list of additional services that will
  either be appended to the end of the node initialization process
  (simple,) or (based on name) replace one of the default services
  (MVP for mempool only.)

- Move components from `p2p` into `types` to support writing
  novel services/mempool implementations.
