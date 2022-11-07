# RFC 26: Mark peers as bad based on ResponseCheckTx

## Changelog
- Nov 4, 2022: Initial draft (jmalicevic)
- Nov 7, 2022: Updated draft (jmalicevic)

## Abstract

Before adding a transaction to its mempool, a node runs `CheckTx` on it. `CheckTx` returns a non-zero code
in case the transaction fails an application specific check. 

Note that there are valid scenarios in which a transaction will not
pass this check (including nodes getting different transactions at different times, meaning some
of them might be obsolete at the time of the check, state changes upon block execution etc.). 
However, Tendermint users observed that
there are transactions that can never have been or will never be valid. They thus propose
to introduce a special response code for `CheckTx` to indicate this behaviour, and ban the peers
who gossip such transactions. Additionally, users expressed a need for banning peers who
repeadetly send transactions failing `CheckTx`. 

The main goal of this document is to analyse the cases where peers could be banned when they send 
transacations failing `CheckTx`, and provide the exact conditions that a peer and transaction have
to satisfy in order to mark the peer as bad.

This document will also include a proposal for implementing these changes within the mempool, including
potential changes to the existing mempool logic and implementation.


## Background

This work was triggered by issue [#7918](https://github.com/tendermint/tendermint/issues/7918) and a related
discussion in [2018](https://github.com/tendermint/tendermint/issues/2185). Additionally,
there was a [proposal](https://github.com/tendermint/tendermint/issues/6523) 
to disconnect from peers after they send us transactions that constantly fail `CheckTx`.  While 
the actual implementation of an additional response code for `CheckTx` is straight forward there 
are certain things to consider. The questions to answer, along with identified risks will be outlined in 
the discussion. 

### Existing issues and concerns

Before diving into the details, we collected a set of issues opened by various users, arguing for 
this behaviour and explaining their needs. 

- Celestia: [blacklisting peers that repeatedly send bad tx](https://github.com/celestiaorg/celestia-core/issues/867)
  and investigating how [Tendermint treats large Txs](https://github.com/celestiaorg/celestia-core/issues/243)
- BigChainDb: [Handling proposers who propose bad blocks](https://github.com/bigchaindb/BEPs/issues/84)
- IBC relayer: Simulate transactions in the mempool and detect whether they could ever have been valid to 
  [avoid overloading the p2p/mempool system](https://github.com/cosmos/ibc-go/issues/853#issuecomment-1032211020)
 (ToDo check with Adi what is this about, what are valid reasons this can happen and when should we disconnect)
  (relevant code [here](https://github.com/cosmos/ibc-go/blob/a0e59b8e7a2e1305b7b168962e20516ca8c98fad/modules/core/ante/ante.go#L23)) 
- From  Adi: Nodes allow transactions with a wrong `minGas` transaction and gossip them, and other nodes keep rejecting them. (Problem seen
  with relayers)

### Current state of mempool/p2p interaction

Transactions received from a peer are handled within the [`handleMempoolMessage`](https://github.com/tendermint/tendermint/blob/85a584dd2ee3437b117d9f5e894eac70bf09aeef/internal/mempool/reactor.go#L120) routine. 

Currently, the mempool can trigger a disconnect from a peer in the case of the following errors:

  - [Unknown message type](https://github.com/tendermint/tendermint/blob/d704c0a0b6fbc042dcb260dbb766bd83bb140cb7/mempool/v0/reactor.go#L182)

However, disconnecting from a peer is not the same as banning the peer. The p2p layer will close the connecton but 
the peer can reconnect without any penalty, and if it as a persistent peer, a reconnect will be initiated
from the node. 

### Current support for peer banning

Currently, the p2p layer implements only banning peers for an indefinite amount of time, by marking them
as bad (calling the `MarkBad` routine implemented by the `Switch`). The peers are reinstated to the list 
of good peers only if the node requires more peers. They can also not be marked as bad forever (blacklisted). 

The application can also blacklist peers if the 
[`filterPeers`](../../spec/abci/abci%2B%2B_app_requirements.md#peer-filtering) 
config flag is set, by providing a set of peers to ban to Tendermint. 

If the discussion in this RFC deems a different banning schema is needed,
the actual implementation and design of this mechanism will be discussed in a separate RFC.
This mechanism should be generic, designed within the p2p layer and simply provide an interface
for reactors to indicate peers to ban and for how long. It should not involve any mempool
specific design considerations. 

## Discussion

If this feature is to be implemented we need to clearly define the following:
1. If `CheckTx` signals a peer should be banned, how do we know which peer(s) to ban?
2. What does banning a peer mean:
   1. A peer can be simply disconnected from. 
   2. Peer is disconnected from and banned.
   3. Conditions for the peer to be banned.
3. Are there possible attack scenarios by allowing this. 

Any further mentions of `banning` will be agnostic to the actual way banning is implemented by the p2p layer. 

### 1. Choosing the peer to ban

Each transaction gossiped contains the ID of the peer that sent that transaction. Upon receiving a transaction,
a node saves the peer ID of the peer(s) that have sent it. As each peer had to have
run `CheckTx` on this transaction before adding it to its own mempool, we can assume this peer
can be held accountable for the validity of transactions it gossips. 

For transactions submitted via `boradcastTxCommit`, this field is empty. 

**Note**  Do we have mechanisms in place to handle cases when `broadcastTxCommit` submits
failing transactions (can this be a form of attack?)?

### 2. What does banning a peer mean 

Tendermint recognizes that peers can accept transactions into their mempool as valid but then those transactions
become invalid, or were valid at some point. 
But, if a transaction could never have been valid, how can it pass a `CheckTx` at any node? What are the valid
(non malicious reasons for such failures). 
This differentiates two scenarios - a) where `CheckTx` fails due to reasons already 
known and b) where `CheckTx` deems a transactions could never have been valid.

For the sake of simplicity, in the remainder of the text we will distinguish the failures due to a) with failures 
signaled with `responseCode = 1` and the failures described in b), failures with `responseCode > 1`.

For a) a peer sends transactions that repeatedly fail CheckTx with `ResponseCheckTx.code = 1`, and is banned or disconnected from to avoid this. 

For b) we need to understand what is the potential reason a transaction could never have been valid on one node, but passes `CheckTx` on another node. 
We need to understand all the possible scenarios in which this can happen:

1. What happens if a node is misconfigured and allows, for example, very large transactions into the mempool. 
This node would then gossip these transactions and they would always fail on other nodes. 
Is this a scenario where we want nodes to disconnect from this peer and ban it without this peer being neccessarily malicious?
2. Are all other reasons for this to happen sign of malicious behaviour where a node explicitly lies? 
In such cases, peers should not only be banned, but punished as well (if they are validators). 

The later is most likely a potential future optimizations, where for now, we will disconnect and ban the peer regardless of the exact reason such a transaction 
is never valid. 

#### **Banning in case of ResponseCheckTx.code = 1**

If a node sends transactions that fail `CheckTx` with a response code of 1, there might be a valid reason for this behaviour. 
A peer should not be banned the first time this happens, but rather if this happens a predefined number 
of times (`numFailures`) within a time interval (`lastFailure`). This time interval should be reset every `failureResetInterval`ms.  

For each peer, we should have a separate `numFailures` and `lastFailure` variable. There is no need to  have one per transaction. 

Whenever a transaction fails, if the `now - lastFailure <= failureResetInterval`, then we increment the `numFailures` for this particular peer and set the `lastFailure` to `now`. Otherwise, we set `lastFailure` to `now` but do not increment the `numFailures`  variable. The reason behind confirming that not too much time
has passed between two failures is that a node can be allowed to send an invalid transaction from time to time, but this change intends to avoid spam. 

Once the value for `numFailures` for a peer reaches `maxAllowedFailures`, the peer is disconnected from and banned. 

Currently the `ResponseCheckTx` code is check in `resCbFirstTime` of the mempool. 


#### **Banning in case of ResponseCheckTx.code > 1**

If a transaction fails since it could never have been valid, `CheckTx` returns a `ResponseCheckTx.code` 
value greater than 1. In this case, the peer should be disconnected from and banned immediately without keeping count on how often 
this has happened. 

The question is whether this transaction should be kept track of in the mempool? We can still store it in 
the cache so that we don't run `CheckTx` on it agan. 
However, there might not be the need to store all the peers that have sent us this transaction. 

Now, if we want to differentiate further reasons of why this transaction is sent to a node (whether it is a sign of malice or not), 
we might need more information on the actual reason for rejection. This could be done by an additional set of response codes provided by the application. 


### Implementation considerations

To implement this functionality, we need to keep track of the `CheckTx` response code for each transaction. However, the current implementation
of the mempool, adds a transaction to the mempool only once and caches its signature, to avoid running `CheckTx` on transactions already checked. 
Thus when a transaction is received from a peer, if it is in the cache,
`CheckTx` is not ran again, but the peers' ID is addded to the list of peers who sent this particular transaction.
These transactions are rechecked once a block is committed to verify that they are still valid.

Currently there are two ways to implement peer banning based on the result of `CheckTx`:

1. Introduce banning when transactions start to be processed
2. Adapt the recheck logic to support this


**Peer banning when transactions are received**

If a transaction fails `CheckTx` the [first time it is seen](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/mempool/v0/clist_mempool.go#L409),  the peer can be banned right there:

>>mempool/v0/clist_mempool.go#L409
```golang

      if (r.CheckTx.Code == abci.CodeTypeOK) && postCheckErr == nil {
          // Check Tx passed
      } else {
			// ignore bad transaction
          mem.logger.Debug(
            "rejected bad transaction",
            "tx", types.Tx(tx).Hash(),
            "peerID", peerP2PID,
            "res", r,
            "err", postCheckErr,
          )
          mem.metrics.FailedTxs.Add(1)

          mem.banPeer(peerP2PID)

          if !mem.config.KeepInvalidTxsInCache {
            // remove from cache (it might be good later)
            mem.cache.Remove(tx)
          }
		}

```					
The `KeepInvalidTxsInCache` configuration parameter defines whether an invalid transaction stays in cache. 
Currently, even transactions that never could have been valid are kept in the cache. This can be mitigating by 
expanding the condititon in the code above by:

```golang
if !mem.config.KeepInvalidTxsInCache || r.CheckTx.Code == abci.NeverValid {
            // remove from cache (it might be good later)
    mem.cache.Remove(tx)
}
```

However, this code will [never be executed](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/mempool/v0/clist_mempool.go#L239) for transactions not seen for the first time. 
One way to go around this is to implement a valid/invalit bit per transaction within the cache. Currently, transactions themselves do not
store such information. As we expect this scenario to be unlikely, instead of increasing the footprint of all transactions in the cache, 
we can keep a map of transactions that are in the cache, but are invalid. Alternatively, the cache could keep two lists, one for valid, and one for invalid transactions. 
The former would modify the following pieces of code as follows:

```golang

type CListMempool struct {
 // ..existing fields

invalidCachedTx map[types.TxKey]bool
}
```


>>mempool/v0/clist_mempool.go#L239
```golang

if !mem.cache.Push(tx) { // if the transaction already exists in the cache
		// Record a new sender for a tx we've already seen.
		// Note it's possible a tx is still in the cache but no longer in the mempool
		// (eg. after committing a block, txs are removed from mempool but not cache),
		// so we only record the sender for txs still in the mempool.
		if e, ok := mem.txsMap.Load(tx.Key()); ok {
			memTx := e.(*clist.CElement).Value.(*mempoolTx)
			memTx.senders.LoadOrStore(txInfo.SenderID, true)
			// TODO: consider punishing peer for dups,
			// its non-trivial since invalid txs can become valid,
			// but they can spam the same tx with little cost to them atm.
		}

    if _, ok := mem.invalidCachedTx.(tx); ok {

        if val , ok := lastFailure.Load(peerID); ok {
          
        if time.Since(val) <= failureResetInterval {
          numFailures.(peerID) = numFailures.(peerID) + 1
          if numFailures.(peerID) == maxAllowedFailures {
              mem.banPeer(peerID)
            }
          }
        }     
        lastFailure(peerID) = time.Now()
    }
		return mempool.ErrTxInCache
	}

```
If transactions with `ResponseCheckTx.code > 1` are not deleted from the cache and we want to ban them on the first offence,
we can skip the second if in the code above.

**Implementing peer banning on recheck**

Currently the only check within the recheck logic is confirming whether once valid transactions, 
, where `ResponseCheckTx.code == 0`, are still valid. 

As this logic loops through the transactions in any case, we can leverage it to check whether we can ban peers. 
However, this approach has several downsides:
- It is not timely enough. Recheck is executed after a block is committed, leaving room for a bad peer to send 
us transactions the entire time between two blocks. 
- If we want to keep track of when peers sent usa traansaction and punish them only if the misbehaviour happens 
frequently enough, this approach makes it hard to keep track of when exactly was a transaction submitted. 



### Impacted functionalities

- mempool caching
- what happens on PreCheck and PostCheck
- what happens on mempool.Recheck

### References

mempool/v0/mempool.go:L247 - ToDo note on punishing peers for duplicate transactions;

> Links to external materials needed to follow the discussion may be added here.
>
> In addition, if the discussion in a request for comments leads to any design
> decisions, it may be helpful to add links to the ADR documents here after the
> discussion has settled.



- 