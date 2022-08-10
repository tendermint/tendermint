# Address Book

Implemented as part of the `pex` package.

> // AddrBook is an address book used for tracking peers
> // so we can gossip about them to others and select
> // peers to dial.

`AddrBook` is an interface, it extends `service.Service`.
`addrBook` is the (only) implementation of this interface.

The address book stores `knownAddress` instances.

Addresses are added to the address book in the following situations:

1. When a peer address is received via PEX protocol, the source is the sender
   of the PEX message
1. When an inbound peer is added to the PEX reactor, in this case the peer
   itself is set as the source
1. When the switch is instructed to dial addresses via the `DialPeersAsync`
   method, in this case the node itself is set as the source

## New addresses

The `AddAddress` method adds a new address to the address book.

The added address is associated to a *source* address, which identifies the
node from which the address was learned.

An address contains a node ID and a network address (IP and port).
The address book can keep multiple network addresses for the same node ID, but
this number should be limited.
A new address with a node ID that is already present in the address book is
**not** added when:

- The latest address added with the same node ID is stored in an old bucket
- There are addresses for this node ID in `maxNewBucketsPerAddress = 4` buckets
- Randomly, with an exponentially increasing probability as the number of
  buckets in which there is an address with this node ID increases. If the
  address is in one bucket, it is not added with 50% of probability. If it is
  in two buckets, with 75% of probability, and so on.

Addresses of nodes not yet tracked, i..e, whose node IDs that are not present
in the address book, should always be added.

Addresses are stored in [buckets](#buckets).
The bucket to which a new address is added is computed based on the address and
its source address.
An address is not added to a bucket if it is already present there.
This should not happen with addresses with new node IDs, but it can happen with
addresses with known node IDs.
In fact, as the mapping of buckets for new addresses is mainly determined by
the network group of the source address, the same address can be present in
more than one bucket provided that it was added with different source addresses.

The new address is also added to the `addrLookup` table, which keeps the last
address added for each node ID.
This table is used by most of the address book methods (e.g., `HasAddress`,
`IsGood`, `MarkGood`, `MarkAttempt`), as it provides a fast access to address entries.

### Errors

- if the added address or the associated source address are nil
- if the added address is invalid
- if the added address is a node address
- if the added address ID is of a banned peer
- if either the added address or the associated source address IDs are configured as private IDs
- if `routabilityStrict` is set and the address is not routable
- in case of failures computing the bucket for the new address (`calcNewBucket` method)
- if the added address instance, which is a new address, is configured as an
  old address (sanity check of `addToNewBucket` method)

## Need for Addresses

The `NeedMoreAddrs` method verifies whether the address book needs more addresses.

It is invoked by the PEX reactor to define whether to request peer addresses
to a new outbound peer or to a randomly selected connected peer.

The address book needs more addresses when it has less than `1000` addresses
registered, counting all buckets for new and old addresses.

## Good peers

The `MarkGood` method marks a peer ID as good.

It is invoked by the consensus reactor, via switch, when the number of useful
messages received from a peer is a multiple of `10000`.
Vote and block part messages are considered for this number, they must be valid
and not be duplicated messages to be considered useful.

> The `SwitchReporter` type also invokes this method for reasons that appear to
> be the same mentioned above.
> No reactor, however, use this method with those reasons.

The effect of this action is that the address registered for the peer ID in the
`addrLookup` table, which is the last address added with that ID, is marked as
good and moved to a bucket of old addresses.
An address marked as good has its failed to connect counter and timestamps reset.
If the destination bucket of old addresses is full, the oldest address in the
bucket is moved (downgraded) to a bucket of new addresses.

Moving the peer address to a bucket of old addresses has the effect of
upgrading, or increasing the ranking of a peer in the address book.

## Bad peers

The `MarkBad` method marks a peer as bad and bans it for a period of time.

It is invoked by the PEX reactor, with banning time of 24 hours, in the following cases:

- When PEX requests are received too often from a peer
- When an invalid PEX response is received from a peer
- When an unsolicited PEX response is received from a peer
- When the `maxAttemptsToDial` a peer limit (`16`) is reached
- If an `ErrSwitchAuthenticationFailure` error is returned when dialing a peer

The effect of this action is that the address registered for the peer ID in the
`addrLookup` table, which is the last address added with that ID, is banned for
the provided period of time.
The banned peer is removed from the `addrLookup` table and from all buckets
where its addresses are stored.
The banned entries, however, are not discarded, but kept in the `badPeers` map,
indexed by peer ID.
This allows them to be [reinstated](#reinstating-addresses), i.e., to be added
back to the address book, when their ban period expires.

## Dial Attempts

The `MarkAttempt` method registers a failed attempt to connect to an address. 

It is invoked by the PEX reactor when it fails dialing a peer, but the returned
error is not an `ErrSwitchAuthenticationFailure` error, which in its turn
causes the peer to be marked as a bad peer.

The failed connection attempt is registered in the entry associated to the
address ID in the `addrLookup` table.
Its counter of failed `Attempts` is increased and the failure time is
registered in `LastAttempt`.
The possible effect of multiple failed connect attempts is to turn the address
into a *bad* address (do not confuse with banned addresses).
An address becomes bad if it is a new address and when connections attempts:

- Have not been made over a week, i.e., `LastAttempt` is older than a week
- Have failed 3 times and never succeeded, i.e., `LastSucess` field is unset
- Have failed 10 times in the last week, i.e., `LastSucess` is older than a week

Address marked as *bad* are the first removed from a bucket of new addresses
when the bucket becomes full.

## Reinstating addresses

The `ReinstateBadPeers` method attempts to re-add banned addresses to the address book.

It is invoked by the PEX reactor when dialing new peers, provided that more
peer addresses are needed.

The set of banned peer addresses is retrieved from the `badPeers` map.
If the address should not be banned anymore, i.e., its banned time has expired,
the address is added to a bucket of new addresses and removed from the list of
banned addresses.

## Removing addresses

The `RemoveAddress` method remove an address from the address book.

It is invoked by the switch when it dials a peer or accepts a connection from a
peer that end ups being the node itself (`IsSelf` error).
In both cases, the address dialed or accepted is also added to the address book
as a local address, via the `AddOurAddress` method.

The entry registered with the peer ID of the address in the `addrLookup` table,
which is the last address added with that ID, is removed from all buckets where
it is registered and from the `addrLookup` table.

## Pick address

The `PickAddress` method

TODO:

## Random selection

The `GetSelection` method

TODO:

## Random selection with bias

The `GetSelectionWithBias` method

TODO:

## Buckets

Addresses are stored into buckets.
There are buckets for new addresses and buckets for old addresses.
The number of buckets is fixed: there are `256` buckets for new addresses, and
`64` buckets for old addresses, `320` in total.
Each bucket can store up to `64` addresses.

When a bucket is full, one of its entries is removed to give room for a new entry.
The first option is to remove any *bad* address in the bucket.
If none is found, the *oldest* address in the bucket is removed, i.e., the
address with the oldest last attempt to dial.
In the case of a bucket for new addresses, the removed address is dropped.
In the case of a bucket for old addresses, the address is moved to a bucket of
new addresses.

The bucket that stores an `address` is defined by the following two methods,
for new and old addresses:

- `calcNewBucket(address, source) = hash(key + groupKey(source) + hash(key + groupKey(address) + groupKey(source)) % newBucketsPerGroup) % newBucketCount`
- `calcOldBucket(address) = hash(key + groupKey(address) + hash(key + address) % oldBucketsPerGroup) % oldBucketCount`

The `key` is a fixed random 96-bit (8-byte) string.
The `groupKey` for an address is a string representing its network group.
The `source` of an address is the address of the peer from which we learn the
address, it only applies for new addresses.
The first (internal) hash is reduced to an integer up to `newBucketsPerGroup =
32`, for new addresses, and `oldBucketsPerGroup = 4`, for old addresses.
The second (external) hash is reduced to bucket indexes, in the interval from 0
to the number of new (`newBucketCount = 256`) or old (`oldBucketCount = 64`) buckets.

Notice that new addresses with sources from the same network group are more
likely to end up in, therefore to compete for, the same bucket.
For old address, instead, two addresses are more likely to end up in the same
bucket when they belong to the same network group.

## Persistence

TODO:
