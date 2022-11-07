# RFC 26: Mark peers as bad based on ResponseCheckTx

## Changelog
- Nov 4, 2022: Initial draft (jmalicevic)


## Abstract

Before adding a transaction to its mempool, a node runs `CheckTx` on it. `CheckTx` returns a non-zero code
in case `CheckTx` returns a non-zero code. Note that there are valid reasons a transaction will not
pass this check (including nodes getting different transactions at different times, meaning some
of them might be obsolete at the time of the check). However, Tendermint users observed that
there are transactions that can never have been or will never be valid. They thus propose
to introduce a special response code for `CheckTx` to indicate this behaviour. 

The reason behind differentiating between this scenario and other failures is to 
be able to ban and disconnect from peers who gossip such transactions.

This documents outlies the potental scenarios around this behaviour, understanding when this 
happens and its risks, along with potential implementation suggestions.  

## Background

This work was triggered by issue [#7918](https://github.com/tendermint/tendermint/issues/7918). It was 
additionally discussed in [2018](https://github.com/tendermint/tendermint/issues/2185). Additionally,
there was a [proposal](https://github.com/tendermint/tendermint/issues/6523) to disconnect from peers after they send us transactions that constantly fail `CheckTx`  While 
the actual implementation of an additional response code for `CheckTx` is straight forward there 
are certain things to consider. The questions to answer, along with idnetified risks will be outlined in 
the discussion. 

Before diving into the details, we collected a set of issues opened by various users, arguing for 
this behaviour and explaining their needs. 

- Celestia: [blacklisting peers that repeatedly send bad tx](https://github.com/celestiaorg/celestia-core/issues/867)
  and investigating how [Tendermint treats large Txs](https://github.com/celestiaorg/celestia-core/issues/243)
- BigChainDb: [Handling proposers that who propose bad blocks](https://github.com/bigchaindb/BEPs/issues/84)
- IBC relayer: Simulate transactions in the mempool and detect whether they could ever have beein valid to 
  [avoid overloading the p2p/mempool system](https://github.com/cosmos/ibc-go/issues/853#issuecomment-1032211020)
 (ToDo check with Adi what is this about, what are valid reasons this can happen and when should we disconnect)
  (relevant code [here](https://github.com/cosmos/ibc-go/blob/a0e59b8e7a2e1305b7b168962e20516ca8c98fad/modules/core/ante/ante.go#L23)) 


Currently, the mempool can trigger a disconnect from a peer in the case of the following errors:
    - [Unknown message type](https://github.com/tendermint/tendermint/blob/d704c0a0b6fbc042dcb260dbb766bd83bb140cb7/mempool/v0/reactor.go#L182)

However, disconnecting from a peer is not the same as banning the peer. The p2p layer will close the connecton but 
the peer can reconnect without any penalty, and if it as a persistent peer, a reconnect will be initiated
from the node. 

- Transaction handling from [peers](https://github.com/tendermint/tendermint/blob/85a584dd2ee3437b117d9f5e894eac70bf09aeef/internal/mempool/reactor.go#L120)

## Discussion

If this feature is to be implemented we need to clearly define the following:
1. If `CheckTx` signals a peer should be banned, how do we know which peer(s) to ban?
2. What does banning a peer mean:
   1. A peer can be simple disconnected from. 
   2. Peer is disconnected from and banned.
   3. Should a peer be banned based on the first offense?
3. Are there possible attack scenarios by allowing this. 

Note that the actual implementation and design of peer banning will be discussed in a separate RFC 
as it relates to the p2p layer and should be designed in a way that is 
generic and usable by all reactors. This documment will therefore simply refer to peer banning 
regardless of its actual implementation.

Currently, the p2p layer implements only banning peers for an indefinite amount of time, by marking them
as bad (calling the `MarkBad` routine implemented by the `Switch`). The peers are reinstated to the list of good peers only if the node requires more peers. 

Additionally, if the [`filterPeers`](../../spec/abci/abci%2B%2B_app_requirements.md#peer-filtering) config flag is set, the application can ban peers itself. 

### 1. Which peer should be banned

Each transaction gossiped contains the ID of the peer that sent us that transaction. Upon receiving a transaction,
a node saves the peer ID of the peer(s) that have sent it. As each peer had to have
run `CheckTx` on this transaction before adding it to its own mempool, we can assume this peer
can be held accountable for the validity of transactions it gossips. 

However, for transactions received via `boradcastTxCommit`, this field is empty. 

### 2. What does banning a peer mean 

Tendermint recognizes that peers can accept transactions into their mempool as valid but then those transactions
become invalid, what are the valid (non malicious) use cases a peers transaction that could never have been valid passes a local `CheckTx` but fails `CheckTx` on another node? This differentiates two scenarios - a) where `CheckTx` fails due to reasons already 
known and b) where `CheckTx` deems a transactions could never have been valid.

For the sake of simplicity, in the remainder of the text we will distinguish the failures due to a) with failures 
signaled with `responseCode = 1` and the failures described in b), failures with `responseCode > 1`.

For a) a peer sends transactions that repeadetly fail CheckTx with `ResponseCheckTx.code = 1`, and is banned or disconnected from to avoid this. 

For b) we need to understand what is the potential reason a transaction could never have been valid on one node, but passes `CheckTx` on another node. We need to understand all the possible scenarios in which this can happen:

1. What happens if a node is misconfigured and allows, for example, very large transactions into the mempool. This node would then gossip these transactions and they would always fail on other nodes. Is this a scenario where we want nodes to disconnect from this peer and ban it without this peer being neccessarily malicious? 
2. Are all other reasons for this to happen sign of malicious behaviour where a node explicitly lies? In such cases, peers should not only be banned, but punished as well (if they are validators). 


#### **Banning in case of ResponseCheckTx.code = 1**

If a node sends transactions that fail CheckTx with a response code of 1, there might be a valid reason for this behaviour. A peer should be banned the first time this happens, but rather if this happens a predefined number 
of times (`numFailures`) within a time interval (`lastFailure`). This time interval should be reset every `failureResetInterval`.  

For each peer, we should have a separate `numFailures` and `lastFailure` variable. There is no need to  have one per transaction. 

Whenever a transaction fails, if the `now - lastFailure <= failureResetInterval`, then we increment the `numFailures` for this particular peer and set the `lastFailure` to `now`. Otherwise, we set `lastFailure` to `now` but do not increment the `numFailures`  variable. 

Once the value for `numFailures` for a peer reaches `maxAllowedFailures`, the peer is disconnected from and banned. 

Currently the `ResponseCheckTx` code is check in `resCbFirstTime` of the mempool. 

Currently, invalid transactions are still kept in the cache. If a transaction is in cache, `CheckTx` is not ran for it again. Thus we will not discover subsequent offenses by the same peer or different peers submitting invalid transactions. 

One way to go around this is to implement a valid/invalit bit per transaction within the cache and check this 
[in case the transaction is already in the cache](https://github.com/tendermint/tendermint/blob/816c6bac00c63a421a1bdaeccbc081c5346cb0d8/mempool/v0/clist_mempool.go#L244).


#### **Banning in case of ResponseCheckTx.code > 1**

If a transaction fails since it could never have been valid, `CheckTx` returns a `ResponseCheckTx.code` value greater than 1. In this case, the peer should be disconnected from and banned immediately without keeping count on how often 
this has happened. 

The question is whether this transaction should be kept track of in the mempool? We can still store it in the cache so that we don't run `CheckTx` on it agan. However, there might not be the need to store all the peers that have sent us this transaction. 

Now, if we want to differentiate further reasons of why this transaction is sent to a node (whether it is a sign of malice or not), we might need more information on the actual reason for rejection. This could be done by an additional set of response codes provided by the application. 





Questions : What is the rationale in adding the Tx to the cache before running checkTx on it? Why do we leave it in the cache it is invalid?
### References

mempool/v0/mempool.go:L247 - ToDo note on punishing peers for duplicate transactions;

> Links to external materials needed to follow the discussion may be added here.
>
> In addition, if the discussion in a request for comments leads to any design
> decisions, it may be helpful to add links to the ADR documents here after the
> discussion has settled.