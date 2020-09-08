# ADR 059: Go API Stability

## Changelog

- 2020-09-08: Initial version (@erikgrinaker)

## Context

With the release of Tendermint 1.0, we want to adopt [semantic versioning](https://semver.org). One major implication of this is a guarantee that we will not make backwards-incompatible changes until Tendermint 2.0 (except in pre-release versions). In order to provide this guarantee for our Go API, we must clearly define which of our APIs are public, and what changes are considered backwards-incompatible.

Currently, we list packages that we consider public in our [README](https://github.com/tendermint/tendermint#versioning), but since we are still at version 0.x we do not provide any backwards compatiblity guarantees at all.

### Glossary

* **External package:** a different Go package, even a child or sibling package in the same project.

* **External project:** a different Git repository.

* **Exported:** a Go identifier starting with an uppercase letter, which can therefore be accessed by an external package.

* **Go API:** any exported Go identifier.

* **Public API:** a Go API that can be imported into an external project.

## Alternative Approaches

- Split all public APIs out to separate Go modules in separate Git repositories, and consider all Tendermint code internal and not subject to API backwards compatibility at all. This was rejected, since it has been attempted by the Tendermint project earlier, resulting in too much dependency management overhead.

- Simply document which APIs are public, and which are internal. This is the current approach, but external projects appear to ignore this and depend on internal code anyway.

## Decision

From Tendermint 1.0, all internal code will be placed in a root-level [`internal` package](https://golang.org/cmd/go/#hdr-Internal_Directories), which the Go compiler will block for use by external projects. All exported items outside of the `internal` package are considered a public API and subject to backwards compatibility guarantees, except files ending in `_test.go`.

The `crypto` package will be split out to a separate module in a separate repo. This is the main general-purpose package used by external projects, and is the only Tendermint dependency in e.g. IAVL which can cause some problems for projects depending on both IAVL and Tendermint.

The `tm-db` package will remain a separate module in a separate repo.

## Detailed Design

### Public API

TODO: this will list the specific packages that are considered public APIs, and thus placed outside of the `internal` package.

### Backwards-Compatible Changes

In Go, [almost all API changes are backwards-incompatible](https://blog.golang.org/module-compatibility) and thus exported items in public APIs generally cannot be changed until Tendermint 2.0. The only backwards-compatible changes we can make to exported items are:

- Adding a new package.

- Adding a new identifier to the package scope (e.g. const, var, func, struct, interface, etc.).

- Adding a new method to a struct.

- Adding a new field to a struct, where the new field's type does not change the struct [comparability](https://golang.org/ref/spec#Comparison_operators), and the zero-value preserves any old behavior.

- Adding a new method to an interface _if the interface has a private method_ (since this makes it impossible for external programs to implement the interface).

Note that adding methods and fields to a struct may break programs that embed two structs in a struct, causing the promoted method or field to change if there is a conflict - we do _not_ consider this breaking, and users are advised to avoid this.

Also note that public APIs can access private types (e.g. via an exported function, method, or field), in which case the exported fields and methods on these private types are also part of the public API and covered by its backwards compatiblity guarantee. In general, private types should never be accessible via public APIs unless wrapped in an exported interface.

In particular, backwards-incompatible changes that _cannot_ be made include:

- Adding a new struct field of an incomparable type (slice, map, func, or struct containing these) to a comparable struct (that does not already have an incomparable field).

- Adding a new method to an interface (unless the interface contains a private method).

- Changing a function or method signature in any way, including adding a variadic parameter (changing parameter names is fine).

- Upgrading a dependency to a new major version if we return, export, or embed types from the dependency.

### Strategies for API Evolution

The API guarantees above can be fairly constraining, but are unavoidable given the Go language design. The following tricks can be employed where appropriate to allow us to make changes to the API:

- We can add a new function or method with a different name that takes additional parameters, and have the old function call the new one.

- Functions and methods can take an options struct instead of separate parameters, to allow adding new options - this is particularly suitable for functions that take many parameters and are expected to be extended, and especially for interfaces where we cannot add new methods with different parameters at all.

- Interfaces can include a private method, e.g. `interface { private() }`, to make them unimplementable by external packages and thus allow us to add new methods to the interface without breaking other programs. Of course, this can't be used for interfaces that should be implementable externally.

- We can use [interface upgrades](https://avtok.com/2014/11/05/interface-upgrades.html) to allow implementers of an existing interface to also implement a new interface, as long as the old interface can still be used - e.g. the new interface `BetterReader` may have a method `ReadBetter()`, and a function that takes a `Reader` interface as an input can check if the implementer also implements `BetterReader` and in that case call `ReadBetter()` instead of `Read()`.

- Structs can include a hidden field of an incomparable type, e.g. `struct { _ [0]func() }`, to prevent external programs from comparing the struct, thus allowing us to add new fields of an incomparable type (slice, map, func, or struct containing these).

## Status

Proposed

## Consequences

### Positive

### Negative

### Neutral

## References

- [#4451: Place internal APIs under internal package](https://github.com/tendermint/tendermint/issues/4451)
