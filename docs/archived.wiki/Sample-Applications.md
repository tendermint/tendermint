## Name Resolution

One of the best ways to understand how to build applications on Tendermint is to dive into the implementation of `NameTx` (See [1](https://github.com/tendermint/tendermint/blob/develop/types/tx.go#L205), [2](https://github.com/tendermint/tendermint/blob/develop/state/execution.go#L514), and also [DNS](https://github.com/eris-ltd/mindy)).

The most interesting thing about name-resolution applications on Tendermint is the speed and security offered by Tendermint's [[Light Client Protocol]].

## Etc
- [DNS](https://github.com/eris-ltd/mindy)
- [[Payment Channels]]
- [[Actually Secure Certificate Authority]]
- [[Mesh Networks]]