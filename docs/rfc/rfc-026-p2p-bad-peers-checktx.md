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
Note that, currently, the p2p layer implements only banning peers for an indefinite amount of time, by marking them
as bad. Additionally, if the [`filterPeers`](../../spec/abci/abci%2B%2B_app_requirements.md#peer-filtering) config flag is set, the application can ban peers itself. 

- Transaction handling from [peers](https://github.com/tendermint/tendermint/blob/85a584dd2ee3437b117d9f5e894eac70bf09aeef/internal/mempool/reactor.go#L120)

## Discussion

If this feature is to be implemented we need to clearly define the following:
1. If `CheckTx` signals a peer should be banned, how do we know which peer(s) to ban?
2. Should a peer be banned based on the first offense?
3. What does banning a peer mean:
   1. A peer can be simple disconnected from. 
   2. Peer is disconnected from and banned by marking the peer as bad (calling `MarkBad`).
   3. Peer is disconnected from and banned temporarily (currently not implemented). 
4. Are there possible attack scenarios by allowing this. 

Note that the actual peer banning will be discussed in a separate RFC as it relates to
the p2p layer and should be designed in a way that is generic and usable by all reactors. 

### 1. Which peer should be banned

Each transaction gossiped contains the ID of the peer that sent us that transaction. As each peer had to have
run `CheckTx` on this transaction before adding it to its own mempool, we can assume this peer
can be held accountable for the validity of transactions it gossips. 

As Tendermint recognizes that peers can accept transactions into their mempool as valid but then those transactions
become invalid, what are the valid (non malicious) use cases a peers transaction can pass a local `CheckTx` but fail 
`CheckTx` on another node? This differentiates two scenarios - a) where `CheckTx` fails due to reasons already 
known and b) where `CheckTx` deems a transactions could never have been valid:

For a) a peer sends transaction that repeadetly fail CheckTx (not neccessarily due to the reason discussed in the abstract), and is banned or disconnected from to avoid this. 
Risks: What if this behaviour exists due to a bad confguration and we have all peers sending and accepting transactions that are for example too large? We will end up with no peers? 
For b) we need to understand what is the potential reason a transaction could never have been valid on one node, but passes `CheckTx` on another node. It seems that the only reason this happens is if a node is malicious and explicitly lies. In such cases, peers should not only be banned, but punished as well. 

For each transaction a peer keeps track of the peer it has received the transaction from so they can all be banned. 

mempool/v0/mempool.go:L247 - ToDo note on punishing peers for duplicate transactions;
TxInfo.sender contains the SenderID - for transactions added via broadcastTxCommit this is empty


### References

> Links to external materials needed to follow the discussion may be added here.
>
> In addition, if the discussion in a request for comments leads to any design
> decisions, it may be helpful to add links to the ADR documents here after the
> discussion has settled.