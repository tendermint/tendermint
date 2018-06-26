# Transactional Semantics

In [Using Tendermint](./using-tendermint.md#broadcast-api) we
discussed different API endpoints for sending transactions and
differences between them.

What we have not yet covered is transactional semantics.

When you send a transaction using one of the available methods, it first
goes to the mempool. Currently, it does not provide strong guarantees
like "if the transaction were accepted, it would be eventually included
in a block (given CheckTx passes)."

For instance a tx could enter the mempool, but before it can be sent to
peers the node crashes.

We are planning to provide such guarantees by using a WAL and replaying
transactions (See
[this issue](https://github.com/tendermint/tendermint/issues/248)), but
it's non-trivial to do this all efficiently.

The temporary solution is for clients to monitor the node and resubmit
transaction(s) and/or send them to more nodes at once, so the
probability of all of them crashing at the same time and losing the msg
decreases substantially.
