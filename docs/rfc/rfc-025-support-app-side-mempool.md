# RFC 25: Support Application Defined Transaction Storage (app-side mempools)

## Changelog

- Aug 17, 2022: initial draft (@williambanfield)

## Abstract

With the release of ABCI++, specifically the `PrepareProposal` call, the utility of the Tendermint mempool becomes much less clear.
This RFC discusses possible changes that should be considered to Tendermint to better support applications that intend to use
`PrepareProposal` to implement much more powerful transaction ordering and filtering functionality than Tendermint can provide.

## Background

* TM has a mempool
* Mempool provides some basic validation and ordering. V1 provides prioritization
* Validation and ordering is primitive. Cannot fit the needs of many different applications.
* Many applications wish to implement much more advanced transaction ordering and validation semantics.
* TM cannot support the very diverse set of requirements desired by all of its different users.
* Tm should make it easy for developers to experiment with different transaction.
* p2p gossip network _is_ Tendermint's responsibility.
* We must devise a way to allow application to developers to control which messages
* are still valid to gossip while not leaking too much of the abstraction of
* this transaction buffer.

## Discussion

Rename to GossipList

### Start Gossiping

* Transactions still arrive _at_ Tendermint and are passed to the app via `checktx` or similar.

### Stop Gossiping

* Need way for application to tell tendermint what to stop gossiping
* current mechanism is recheck tx.

### Replace

* TM gossip list may become full.
* If new Tx arrives at Tm, how do we allow the application to replace that
* transaction in the gossip list?
* Do we drop by default? That seems pretty wrong.
* Maybe we hand the application the entire transaction list each time?
* How big do we generally allow the mempool to become now?  1GB default. 
* Strikes me as too big to constantly communicate on every check tx call.
* We could deliver the current size and max size on each request.
* This would alert the application that it must remove some set of transactions in order to 
* keep the one it sees now. As long as we guarantee that there are no other ways that a tx
* can leave the list, then the application can easily know the list contents exactly by id
* application must provide ID that is unique.

### Persistence

* Should we persist the list and give it to the application? No. We are just for
* gossiping. This does mean that, if the application wants to save transactions it
* needs to hand them all back to Tendermint when it starts back up again. Again,
* light weight in memory only data structure.

> This section contains the core of the discussion.
>
> There is no fixed format for this section, but ideally changes to this
> section should be updated before merging to reflect any discussion that took
> place on the PR that made those changes.

### References

> Links to external materials needed to follow the discussion may be added here.
>
> In addition, if the discussion in a request for comments leads to any design
> decisions, it may be helpful to add links to the ADR documents here after the
> discussion has settled.
