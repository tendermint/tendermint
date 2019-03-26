# Mempool

## Transaction ordering

Currently, there's no ordering of transactions other than the order they've
arrived (via RPC or from other nodes).

So the only way to specify the order is to send them to a single node.

valA:
  - tx1
  - tx2
  - tx3

If the transactions are split up across different nodes, there's no way to
ensure they are processed in the expected order.

valA:
  - tx1
  - tx2

valB:
  - tx3

If valB is the proposer, the order might be:

  - tx3
  - tx1
  - tx2

If valA is the proposer, the order might be:

  - tx1
  - tx2
  - tx3

That said, if the transactions contain some internal value, like an
order/nonce/sequence number, the application can reject transactions that are
out of order. So if a node receives tx3, then tx1, it can reject tx3 and then
accept tx1. The sender can then retry sending tx3, which should probably be
rejected until the node has seen tx2.
