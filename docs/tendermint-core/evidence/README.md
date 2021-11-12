---
order: 1
parent:
  title: Evidence
  order: 3
---

Evidence is used to identify validators who have or are acting malicious. There are multiple types of evidence, to read more on the evidence types please see [Evidence Types](https://docs.tendermint.com/master/spec/core/data_structures.html#evidence).

The evidence reactor works similar to the mempool reactor. When evidence is observed, it is sent to all the peers in a repetitive manner. This ensures evidence is sent to as many people as possible to avoid sensoring. After evidence is received by peers and committed in a block it is pruned from the evidence module.

Sending incorrectly encoded data or data exceeding `maxMsgSize` will result
in stopping the peer.
