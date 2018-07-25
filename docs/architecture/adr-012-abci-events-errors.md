# ADR 012: ABCI Events & String Errors

## Changelog

- *2018-07-12* Initial version

## Context

ABCI tags are intended as a transaction query mechanism. See [ADR 002](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-002-event-subscription.md) for further details.

ABCI errors should serve to provide an abstraction between the application details and the client interface responsible for formatting errors & displaying them to the user.

## Decision

## Status

Proposed

## Consequences

### Positive

- Ability to track distinct events separate from ABCI calls (DeliverTx/BeginBlock/EndBlock)
- More powerful query abilities
- Codespacing rendered unnecessary

### Negative

- More complex query syntax
- More complex search implementation
- Error field has some overlap with error code

### Neutral
