# ADR 023: ABCI Codespaces

## Changelog

- *2018-09-01* Initial version

## Context

ABCI errors should provide an abstraction between application details
and the client interface responsible for formatting & displaying errors to the user.

Currently, this abstraction consists of a single integer (the `code`), where any
`code > 0` is considered an error (ie. invalid transaction) and all type
information about the error is contained in the code. This integer is
expected to be decoded by the client into a known error string, where any
more specific data is contained in the `data`.

In a [previous conversation](https://github.com/tendermint/abci/issues/165#issuecomment-353704015),
it was suggested that not all non-zero codes need to be errors, hence why it's called `code` and not `error code`.
It is unclear exactly how the semantics of the `code` field will evolve, though
better lite-client proofs (like discussed for tags
[here](https://github.com/tendermint/tendermint/issues/1007#issuecomment-413917763))
may play a role.

For now, note that having all type information in a single integer
places an unfortunate requirement on all users of the `code` system to
co-ordinate up-front on which `code` means what, hence restricting their ability experiment
with codes.

For instance, while trying to build an ecosystem of modules that can be composed into a single
ABCI application, the Cosmos-SDK had to hack a higher level "codespace" into the
single integer so that each module could have its own space to express its
errors.

## Decision

Include a `string code_space` in all ABCI messages that have a `code`.
This allows applications to namespace the codes so they can experiment with
their own code schemes.

It is the responsibility of applications to limit the size of the `code_space`
string.

How the codespace is hashed into block headers (ie. so it can be queried
efficiently by lite clients) is left for a separate ADR.

## Consequences

## Positive

- No need for complex codespacing on a single integer
- More expressive type system for errors

## Negative

- Another field in the response needs to be accounted for
- Some redundancy with `code` field
- May encourage more error/code type info to move to the `codespace` string, which
  could impact lite clients.

