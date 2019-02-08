## Design goals

The design goals for Tendermint (and the SDK and related libraries) are:

 * Simplicity and Legibility
 * Parallel performance, namely ability to utilize multicore architecture
 * Ability to evolve the codebase bug-free
 * Debuggability
 * Complete correctness that considers all edge cases, esp in concurrency
 * Future-proof modular architecture, message protocol, APIs, and encapsulation


### Justification

Legibility is key to maintaining bug-free software as it evolves toward more
optimizations, more ease of debugging, and additional features.

It is too easy to introduce bugs over time by replacing lines of code with
those that may panic, which means ideally locks are unlocked by defer
statements.

For example,

```go
func (obj *MyObj) something() {
	mtx.Lock()
	obj.something = other
	mtx.Unlock()
}
```

It is too easy to refactor the codebase in the future to replace `other` with
`other.String()` for example, and this may introduce a bug that causes a
deadlock.  So as much as reasonably possible, we need to be using defer
statements, even though it introduces additional overhead.

If it is necessary to optimize the unlocking of mutex locks, the solution is
more modularity via smaller functions, so that defer'd unlocks are scoped
within a smaller function.

Similarly, idiomatic for-loops should always be preferred over those that use
custom counters, because it is too easy to evolve the body of a for-loop to
become more complicated over time, and it becomes more and more difficult to
assess the correctness of such a for-loop by visual inspection.


### On performance

It doesn't matter whether there are alternative implementations that are 2x or
3x more performant, when the software doesn't work, deadlocks, or if bugs
cannot be debugged.  By taking advantage of multicore concurrency, the
Tendermint implementation will at least be an order of magnitude within the
range of what is theoretically possible.  The design philosophy of Tendermint,
and the choice of Go as implementation language, is designed to make Tendermint
implementation the standard specification for concurrent BFT software.

By focusing on the message protocols (e.g. ABCI, p2p messages), and
encapsulation e.g. IAVL module, (relatively) independent reactors, we are both
implementing a standard implementation to be used as the specification for
future implementations in more optimizable languages like Rust, Java, and C++;
as well as creating sufficiently performant software. Tendermint Core will
never be as fast as future implementations of the Tendermint Spec, because Go
isn't designed to be as fast as possible.  The advantage of using Go is that we
can develop the whole stack of modular components **faster** than in other
languages.

Furthermore, the real bottleneck is in the application layer, and it isn't
necessary to support more than a sufficiently decentralized set of validators
(e.g. 100 ~ 300 validators is sufficient, with delegated bonded PoS).

Instead of optimizing Tendermint performance down to the metal, lets focus on
optimizing on other matters, namely ability to push feature complete software
that works well enough, can be debugged and maintained, and can serve as a spec
for future implementations.


### On encapsulation

In order to create maintainable, forward-optimizable software, it is critical
to develop well-encapsulated objects that have well understood properties, and
to re-use these easy-to-use-correctly components as building blocks for further
encapsulated meta-objects.

For example, mutexes are cheap enough for Tendermint's design goals when there
isn't goroutine contention, so it is encouraged to create concurrency safe
structures with struct-level mutexes.  If they are used in the context of
non-concurrent logic, then the performance is good enough.  If they are used in
the context of concurrent logic, then it will still perform correctly.

Examples of this design principle can be seen in the types.ValidatorSet struct,
and the cmn.Rand struct.  It's one single struct declaration that can be used
in both concurrent and non-concurrent logic, and due to its well encapsulation,
it's easy to get the usage of the mutex right.

#### example: cmn.Rand:

`The default Source is safe for concurrent use by multiple goroutines, but
Sources created by NewSource are not`.  The reason why the default
package-level source is safe for concurrent use is because it is protected (see
`lockedSource` in https://golang.org/src/math/rand/rand.go).

But we shouldn't rely on the global source, we should be creating our own
Rand/Source instances and using them, especially for determinism in testing.
So it is reasonable to have cmn.Rand be protected by a mutex.  Whether we want
our own implementation of Rand is another question, but the answer there is
also in the affirmative.  Sometimes you want to know where Rand is being used
in your code, so it becomes a simple matter of dropping in a log statement to
inject inspectability into Rand usage.  Also, it is nice to be able to extend
the functionality of Rand with custom methods.  For these reasons, and for the
reasons which is outlined in this design philosophy document, we should
continue to use the cmn.Rand object, with mutex protection.

Another key aspect of good encapsulation is the choice of exposed vs unexposed
methods.  It should be clear to the reader of the code, which methods are
intended to be used in what context, and what safe usage is.  Part of this is
solved by hiding methods via unexported methods.  Another part of this is
naming conventions on the methods (e.g. underscores) with good documentation,
and code organization.  If there are too many exposed methods and it isn't
clear what methods have what side effects, then there is something wrong about
the design of abstractions that should be revisited.


### On concurrency

In order for Tendermint to remain relevant in the years to come, it is vital
for Tendermint to take advantage of multicore architectures.  Due to the nature
of the problem, namely consensus across a concurrent p2p gossip network, and to
handle RPC requests for a large number of consuming subscribers, it is
unavoidable for Tendermint development to require expertise in concurrency
design, especially when it comes to the reactor design, and also for RPC
request handling.


## Guidelines

Here are some guidelines for designing for (sufficient) performance and concurrency:

 * Mutex locks are cheap enough when there isn't contention.
 * Do not optimize code without analytical or observed proof that it is in a hot path.
 * Don't over-use channels when mutex locks w/ encapsulation are sufficient.
 * The need to drain channels are often a hint of unconsidered edge cases.
 * The creation of O(N) one-off goroutines is generally technical debt that
   needs to get addressed sooner than later.  Avoid creating too many
goroutines as a patch around incomplete concurrency design, or at least be
aware of the debt and do not invest in the debt.  On the other hand, Tendermint
is designed to have a limited number of peers (e.g. 10 or 20), so the creation
of O(C) goroutines per O(P) peers is still O(C\*P=constant).
  * Use defer statements to unlock as much as possible.  If you want to unlock sooner,
    try to create more modular functions that do make use of defer statements.

## Matras

* Premature optimization kills
* Readability is paramount
* Beautiful is better than fast.
* In the face of ambiguity, refuse the temptation to guess.
* In the face of bugs, refuse the temptation to cover the bug.
* There should be one-- and preferably only one --obvious way to do it.
