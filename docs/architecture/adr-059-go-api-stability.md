# ADR 059: Go API Stability

## Changelog

- 2020-09-08: Initial version. (@erikgrinaker)

- 2020-09-09: Allow changing struct comparability, adding variadic parameters, changing struct field order, and widening named numeric types. Expand glossary and clarify terms. (@erikgrinaker)

## Context

With the release of Tendermint 1.0, we want to adopt [semantic versioning](https://semver.org). One major implication of this is a guarantee that we will not make backwards-incompatible changes until Tendermint 2.0 (except in pre-release versions). In order to provide this guarantee for our Go API, we must clearly define which of our APIs are public, and what changes are considered backwards-compatible.

Currently, we list packages that we consider public in our [README](https://github.com/tendermint/tendermint#versioning), but since we are still at version 0.x we do not provide any backwards compatiblity guarantees at all.

### Glossary

* **External project:** a different Git/VCS repository or code base.

* **External package:** a different Go package, can be a child or sibling package in the same project.

* **Internal code:** code not meant for use in external projects.

* **Internal directory:** code under `internal/` which cannot be imported in external projects.

* **Exported:** a Go identifier starting with an uppercase letter, which can therefore be accessed by an external package.

* **Private:** a Go identifier starting with a lowercase letter, which therefore cannot be accessed by an external package unless via an exported field, variable, or function/method return value.

* **Public API:** any Go identifier that can be imported or accessed by an external project, except test code in `_test.go` files.

* **Private API:** any private Go identifier that is not accessible via a public API, and all code in the internal directory.

## Alternative Approaches

- Split all public APIs out to separate Go modules in separate Git repositories, and consider all Tendermint code internal and not subject to API backwards compatibility at all. This was rejected, since it has been attempted by the Tendermint project earlier, resulting in too much dependency management overhead.

- Simply document which APIs are public, and which are internal. This is the current approach, but external projects appear to ignore this and depend on internal code anyway.

## Decision

From Tendermint 1.0, all internal code (except private APIs) will be placed in a root-level [`internal` directory](https://golang.org/cmd/go/#hdr-Internal_Directories), which the Go compiler will block for use by external projects. All exported items outside of the `internal` directory are considered a public API and subject to backwards compatibility guarantees, except files ending in `_test.go`.

The `crypto` package will be split out to a separate module in a separate repo. This is the main general-purpose package used by external projects, and is the only Tendermint dependency in e.g. IAVL which can cause some problems for projects depending on both IAVL and Tendermint.

The `tm-db` package will remain a separate module in a separate repo.

## Detailed Design

### Public API

TODO: this will list the specific packages that are considered public APIs, and thus placed outside of the `internal` directory.

### Backwards-Compatible Changes

In Go, [almost all API changes are backwards-incompatible](https://blog.golang.org/module-compatibility) and thus exported items in public APIs generally cannot be changed until Tendermint 2.0. The only backwards-compatible changes we can make to exported items are:

- Adding a new identifier to the package scope (e.g. const, var, func, struct, interface, etc.).

- Adding a new method to a struct.

- Adding a new field to a struct, if the zero-value preserves any old behavior.

- Changing the order of fields in a struct.

- Adding a variadic parameter to a named function or struct method.

- Adding a new method to an interface, or a variadic parameter to an interface method, _if the interface has a private method_ (which prevents external packages from implementing it).

- Widening a numeric type as long as it is a named type (e.g. `type Number int32` can change to `int64`, but not `int8` or `uint32`).

Note that public APIs can expose private types (e.g. via an exported variable, field, or function/method return value), in which case the exported fields and methods on these private types are also part of the public API and covered by its backwards compatiblity guarantees. In general, private types should never be accessible via public APIs unless wrapped in an exported interface.

Also note that if we accept, return, export, or embed types from a dependency, we assume the backwards compatibility responsibility for that dependency, and must make sure any dependency upgrades comply with the above constraints.

We should run linters on CI for minor version branches to enforce the above constraints. Examples include [breakcheck](https://github.com/gbbr/breakcheck), [apidiff](https://pkg.go.dev/golang.org/x/tools/internal/apidiff?tab=doc), and [apicombat](https://github.com/bradleyfalzon/apicompat).

#### Accepted Breakage

The above changes can still break programs in a few ways - these are _not_ considered backwards-incompatible, and users are advised to avoid this usage:

- If a program uses unkeyed struct literals (e.g. `Foo{"bar", "baz"}`) and we add fields or change the field order, the program will no longer compile or may have logic errors.

- If a program embeds two structs in a struct, and we add a new field or method to an embedded Tendermint struct which also exists in the other embedded struct, the program will no longer compile.

- If a program compares two structs (e.g. with `==`), and we add a new field of an incomparable type (slice, map, func, or struct that contains these) to a Tendermint struct which is compared, the program will no longer compile.

- If a program assigns a Tendermint function to an identifier, and we add a variadic parameter to the function signature, the program will no longer compile.

### Strategies for API Evolution

The API guarantees above can be fairly constraining, but are unavoidable given the Go language design. The following tricks can be employed where appropriate to allow us to make changes to the API:

- We can add a new function or method with a different name that takes additional parameters, and have the old function call the new one.

- Functions and methods can take an options struct instead of separate parameters, to allow adding new options - this is particularly suitable for functions that take many parameters and are expected to be extended, and especially for interfaces where we cannot add new methods with different parameters at all.

- Interfaces can include a private method, e.g. `interface { private() }`, to make them unimplementable by external packages and thus allow us to add new methods to the interface without breaking other programs. Of course, this can't be used for interfaces that should be implementable externally.

- We can use [interface upgrades](https://avtok.com/2014/11/05/interface-upgrades.html) to allow implementers of an existing interface to also implement a new interface, as long as the old interface can still be used - e.g. the new interface `BetterReader` may have a method `ReadBetter()`, and a function that takes a `Reader` interface as an input can check if the implementer also implements `BetterReader` and in that case call `ReadBetter()` instead of `Read()`.

## Status

Proposed

## Consequences

### Positive

### Negative

### Neutral

## References

- [#4451: Place internal APIs under internal package](https://github.com/tendermint/tendermint/issues/4451)
