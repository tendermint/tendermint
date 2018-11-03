# ADR 035: Application custom data inside the block header

## Changelog

## Context
At the moment block header stores application hash (AppHash). It will be very useful if block header can store a custom data (AppData) related to a specific block next to the application hash.
For example AppData can be the block fee (rewards) or even it can be some flags related to that specific block. This information should be deterministic.

There is an old Pull request related to this ADR [here](https://github.com/tendermint/tendermint/pull/1953).

## Decision

## Status

## Consequences

### Positive

### Negative

### Neutral
