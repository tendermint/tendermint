# ADR 076: Combine Spec and Tendermint Repositories 

## Changelog

- 2022-02-04: Initial Draft. (@tychoish)

## Status

Accepted.

## Context

While the specification for tendermint was original in the same
repository as the Go implementation, at some point the specification
was split from the core tendermint repository and maintained
separately from the implementation. While this makes sense in
promoting a conceptual separation of specification and implementation,
in practice this separation was a premature optimization apparently
aimed at supporting alternate implementations of tendermint. 

However, maintaining a spec repo has ended up being a premature
optimization: there are no current projects to develop alternate
implementations of tendermint based on the common specification, and
having separate repositories creates an ongoing maintenance burden to
support versioning and publication.

## Decision

The specification repository should be merged

Stakeholders including representatives from the maintainers of the
spec, the go implementation, and the tendermint rust library, agreed
to merge the repositories in the tendermint core dev meeting on 27
January 2022.

## Alternative Approaches

The main alternative to this was *do nothing* and continue to maintain
separate versioning for both repositories, complicating the release
process for both repositories and creating a difficult to resolve 

## Detailed Design

Within the `tendermint` repository, do the following to merge the
repositories keeping history: 

```bash
git remote add spec git@github.com:tendermint/spec.git
git fetch spec
git checkout -b spec-master spec/master
mkdir spec
git ls-tree -z --name-only HEAD | xargs -0 -I {} git mv {} subdir/
git commit -m "spec: organize specification prior to merge"
git checkout -b spec-merge-mainline origin/master
git merge --allow-unrelated-histories spec-master
```

This branch should be merged into the `tendermint/tendermint`
repository as a normal branch. This commit can also be backported to
the 0.35 branch, if needed.

Then, make a final change to the specification repository adding a
redirect to the README and marking the repository as archived. All
issues should be recreated in the `tendermint/tendermint` repository.

## Consequences

### Positive

Easier maintenance for the specification will obviate a number of
complicated and annoying versioning problem, and will help prevent the
possibility where the specification drifts from the implementation.

Additionally, collocating the specification will help encourage
cross-polenation  and collaboration between engineers focusing on the
specification and the protocol and engineers focusing on the
implementation.

### Negative

Co-locating the spec and go implementation has the potential effect of
prioritizing the Go implementation with regards to the spec, and
making it difficult to think about alternate implementations of the
tendermint algorithm. Although we may want to foster additional
tendermint implementations in the future, this isn't a going concern
in our roadmap at the moment, and *not* merging these repos doesn't
change the fact that the Go implementation of tendermint is already the
primary implementation.

### Neutral

N/A

## References

- https://github.com/tendermint/spec
- https://github.com/tendermint/tendermint
