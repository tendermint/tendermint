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
   itself is set as the source of its address
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
**not added** when:

- The last address added with the same node ID is stored in an old bucket, so
  it is considered a "good" address
- There are addresses associated to the same node ID stored in
  `maxNewBucketsPerAddress = 4` distinct buckets
- Randomly, with a probability that increases exponentially with the number of
  buckets in which there is an address with the same node ID.
  So, a new address for a node ID which is already present in one bucket is
  added with 50% of probability; if the node ID is present in two buckets, the
  probability decreases to 25%; and if it is present in three buckets, the
  probability is 12.5%.

Addresses are stored in [buckets](#buckets).
The bucket to which a new address is added is computed based on the address
itself and its source address.
An address is not added to a bucket if it is already present there.
This should not happen with addresses with new node IDs, but it can happen with
addresses with known node IDs.
In fact, as the mapping of buckets for new addresses is mainly determined by
the network group of the source address, the same address can be present in
more than one bucket provided that it was added with different source addresses.

The new address is also added to the `addrLookup` table, which stores
`knownAddress` entries indexed by their node IDs.
If the new address is from an unknown peer, a new entry is added to the
`addrLookup` table; otherwise, the existing entry is updated with the new
address.
Entries of this table contain, among other fields, the list of buckets where
addresses of a peer are stored.
The `addrLookup` table is used by most of the address book methods (e.g.,
`HasAddress`, `IsGood`, `MarkGood`, `MarkAttempt`), as it provides fast access
to addresses.

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

> The `SwitchReporter` type of `behaviour` package also invokes the `MarkGood`
> method when a "reason" associated with consensus votes and block parts is
> reported.
> No reactor, however, currently provides these "reasons" to the `SwitchReporter`.

The effect of this action is that the address registered for the peer's ID in the
`addrLookup` table, which is the last address added with that ID, is marked as
good and moved to a bucket of old addresses.
An address marked as good has its failed to connect counter and timestamp reset.
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
- When the `maxAttemptsToDial` a limit (`16`) is reached for a peer
- If an `ErrSwitchAuthenticationFailure` error is returned when dialing a peer

The effect of this action is that the address registered for the peer's ID in the
`addrLookup` table, which is the last address added with that ID, is banned for
a period of time.
The banned peer is removed from the `addrLookup` table and from all buckets
where its addresses are stored.

The information about banned peers, however, is not discarded.
It is maintained in the `badPeers` map, indexed by peer ID.
This allows, in particular, addresses of banned peers to be
[reinstated](#reinstating-addresses), i.e., to be added
back to the address book, when their ban period expires.

## Dial Attempts

The `MarkAttempt` method records a failed attempt to connect to an address. 

It is invoked by the PEX reactor when it fails dialing a peer, but the failure
is not in the authentication step (`ErrSwitchAuthenticationFailure` error).
In case of authentication errors, the peer is instead marked as a [bad peer](#bad-peers).

The failed connection attempt is recorded in the address registered for the
peer's ID in the `addrLookup` table, which is the last address added with that ID.
The known address' counter of failed `Attempts` is increased and the failure
time is registered in `LastAttempt`.

The possible effect of recording multiple failed connect attempts to a peer is
to turn its address into a *bad* address (do not confuse with banned addresses).
A known address becomes bad if it is stored in buckets of new addresses, and
when connections attempts:

- Have not been made over a week, i.e., `LastAttempt` is older than a week
- Have failed 3 times and never succeeded, i.e., `LastSucess` field is unset
- Have failed 10 times in the last week, i.e., `LastSucess` is older than a week

Addresses marked as *bad* are the first candidates to be removed from a bucket of
new addresses when the bucket becomes full.

> Note that failed connection attempts are reported for a peer address, but in
> fact the address book records them for a peer.
>
> More precisely, failed connection attempts are recorded in the entry of  the
> `addrLookup` table with reported peer ID, which contains the last address
> added for that node ID, which is not necessarily the reported peer address.

## Reinstating addresses

The `ReinstateBadPeers` method attempts to re-add banned addresses to the address book.

It is invoked by the PEX reactor when dialing new peers.
This action is taken before requesting additional addresses to peers,
in the case that the node needs more peer addresses.

The set of banned peer addresses is retrieved from the `badPeers` map.
Addresses that are not any longer banned, i.e., which banned period has expired,
are added back to the address book as new addresses, while the corresponding
node IDs are removed from the `badPeers` map.

## Removing addresses

The `RemoveAddress` method removes an address from the address book.

It is invoked by the switch when it dials a peer or accepts a connection from a
peer that end ups being the node itself (`IsSelf` error).
In both cases, the address dialed or accepted is also added to the address book
as a local address, via the `AddOurAddress` method.

The entry registered with the peer ID of the address in the `addrLookup` table,
which is the last address added with that ID, is removed from all buckets where
it is stored and from the `addrLookup` table.

> FIXME: is it possible that addresses with the same ID as the removed address,
> but with distinct network addresses, are kept in buckets of the address book?
> While they will not be accessible anymore, as there is no reference to them
> in the `addrLookup`, they will still be there.

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
