# Address Book

**TODO**:

## New and Old buckets

> addToNewBucket() adds a peer to new buckets.
> It is called by:
> - addAddress() method, which is the synchronized version of AddAddress()
> - ReinstateBadPeers()
> - moveToOld(), but as an exceptional case, when  addToOldBucket() fails
> 
> addToOldBucket() adds a peer to old buckets.
> It is  only called by moveToOld() method,
> which is only called by MarkGood() method.
> 
> MarkGood() is only called by the Switch, as part of  MarkPeerAsGood() method.
> 
> The MarkPeerAsGood() switch method is only called by the consensus reactor.
