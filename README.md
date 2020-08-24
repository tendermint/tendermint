# Tendermint Spec

This repository contains specifications for the Tendermint protocol. For the pdf, see the [latest release](https://github.com/tendermint/spec/releases).

There are currently two implementations of the Tendermint protocol,
maintained by two separate-but-collaborative entities:
One in [Go](https://github.com/tendermint/tendermint),
maintained by Interchain GmbH,
and one in [Rust](https://github.com/informalsystems/tendermint-rs),
maintained by Informal Systems.

There have been inadvertent divergences in the specs followed
by the Go implementation and the Rust implementation respectively.
However, we are worked to reconverge these specs into a single unified spec.
Consequently, this repository is in a bit of a state of flux.

At the moment, the spec followed by the Go implementation
(tendermint/tendermint) is in the [spec](spec) directory,
while the spec followed by the Rust implementation
(informalsystems/tendermint-rs) is in the rust-spec
directory. TLA+ specifications are also in the rust-spec directory.

Over time, these specs will converge in the spec directory.
Once they have fully converged, we will version the spec moving forward.
