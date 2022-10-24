# Address Book

The address book tracks information about peers, i.e., about other nodes in the network.

The primary information stored in the address book are peer addresses.
A peer address is composed by a node ID and a network address; a network
address is composed by an IP address or a DNS name plus a port number.
The same node ID can be associated to multiple network addresses.

There are two sources for the addresses stored in the address book.
The [Switch](./switch.md) registers the addresses of peers with which it has
interacted: to which it has dialed or from which it has accepted a connection.
And the [Peer Exchange protocol](./pex-protocol.md) stores in the address book
the peer addresses it discovers, i.e., it learns from connected peers.

There are two entities that retrieve peer addresses from the address book.
The [Peer Manager](./peer_manager.md) retrieves addresses to dial, so to
establish outbound connections.
And the [Peer Exchange protocol](./pex-protocol.md) retrieves random samples of
addresses to offer (send) to peers.

In addition to addresses, the address book also stores some information about
the quality of peers with which the node has interacted.
This includes information about connection attempts, failed or succeeded, with
peers, or with specific network address of a peer.
And information about peers provided by the protocols (reactors), currently
restricted to the definition of [good](#good-peers) or [bad](#bad-peers) peers.
The quality metrics associated to peers influences the selection of peers to
dial and offered via PEX protocol, in a way that they are not fully random.

## Adding addresses

The `AddAddress` method adds the address of a peer to the address book.

The added address is associated to a *source* address, which identifies the
node from which the peer address was learned.

Addresses are added to the address book in the following situations:

1. When a peer address is learned via PEX protocol, having the sender
   of the PEX message as its source
1. When an inbound peer is added, in this case the peer itself is set as the
   source of its own address
1. When the switch is instructed to dial addresses via the `DialPeersAsync`
   method, in this case the node itself is set as the source

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
- if the added address is the local node's address
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

## Pick address

The `PickAddress` method returns an address stored in the address book, chosen
at random with a configurable bias toward new addresses.

It is invoked by the Peer Manager to obtain a peer address to dial, as part of
its `ensurePeers` routine.
The bias starts from 10%, when the peer has no outbound peers, increasing by
10% for each outbound peer the node has, up to 90%, when the node has at least
8 outbound peers.

The configured bias is a parameter that influences the probability of choosing
an address from a bucket of new addresses or from a bucket of old addresses.
A second parameter influencing this choice is the number of new and old
addresses stored in the address book.
In the absence of bias (i.e., if the configured bias is 50%), the probability
of picking a new address is given by the square root of the number of new
addresses divided by the sum of the square roots of the numbers of new and old
addresses.
By adding a bias toward new addresses (i.e., configured bias larger than 50%),
the portion on the sample occupied by the square root of the number of new
addresses increases, while the corresponding portion for old addresses decreases.
As a result, it becomes more likely to pick a new address at random from this sample.

> The use of the square roots softens the impact of disproportional numbers of
> new and old addresses in the address book. This is actually the expected
> scenario, as there are 4 times more buckets for new addresses than buckets
> for old addresses.

Once the type of address, new or old, is defined, a non-empty bucket of this
type is selected at random.
From the selected bucket, an address is chosen at random and returned.
If all buckets of the selected type are empty, no address is returned.

## Random selection

The `GetSelection` method returns a selection of addresses stored in the
address book, with no bias toward new or old addresses.

It is invoked by the PEX protocol to obtain a list of peer addresses with two
purposes:

- To send to a peer in a PEX response, in the case of outbound peers or of
  nodes not operating in seed mode
- To crawl, in the case of nodes operating in seed mode, as part of every
  interaction of the `crawlPeersRoutine`

The selection is a random subset of the peer addresses stored in the
`addrLookup` table, which stores that last address added for each peer ID.
The target size of the selection is `23%` (`getSelectionPercent`) of the
number of addresses stored in the address book, but it should not be lower than
`32` (`minGetSelection`) --- if it is, all addresses in the book are returned
--- nor greater than `250` (`maxGetSelection`).

> The random selection is produced by:
> - Retrieving all entries of the `addrLookup` map, which by definition are
>   returned in random order.
> - Randomly shuffling the retrieved list, using the Fisher-Yates algorithm

## Random selection with bias

The `GetSelectionWithBias` method returns a selection of addresses stored in
the address book, with bias toward new addresses.

It is invoked by the PEX protocol to obtain a list of peer addresses to be sent
to a peer in a PEX response.
This method is only invoked by seed nodes, when replying to a PEX request
received from an inbound peer (i.e., a peer that dialed the seed node).
The bias used in this scenario is hard-coded to 30%, meaning that 70% of
the returned addresses are expected to be old addresses.

The number of addresses that compose the selection is computed in the same way
as for then non-biased random selection.
The bias toward new addresses is implemented by requiring that the configured
bias, interpreted as a percentage, of the select addresses come from buckets of
new addresses, while the remaining come from buckets of old addresses.
Since the number of old addresses is typically lower than the number of new
addresses, it is possible that the address book does not have enough old
addresses to include in the selection.
In this case, additional new addresses are included in the selection.
Thus, the configured bias, in practice, is towards old addresses, not towards
new addresses.

To randomly select addresses of a type, the address book considers all
addresses present in every bucket of that type.
This list of all addresses of a type is randomly shuffled, and the requested
number of addresses are retrieved from the tail of this list.
The returned selection contains, at its beginning, a random selection of new
addresses in random order, followed by a random selection of old addresses, in
random order.

## Dial Attempts

The `MarkAttempt` method records a failed attempt to connect to an address. 

It is invoked by the Peer Manager when it fails dialing a peer, but the failure
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

**Note** In v0.34 a peers is currently marked good only from the consensus reactor 
whenever it delivers a correct consensus message.

## Bad peers

The `MarkBad` method marks a peer as bad and bans it for a period of time.

It is invoked by the [PEX reactor](pex-protocol.md#misbehavior), with banning time of 24 hours, in the following cases:

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

The same logic is also internally used by the address book for removing
addresses of a peer that is [marked as a bad peer](#bad-peers).

The entry registered with the peer ID of the address in the `addrLookup` table,
which is the last address added with that ID, is removed from all buckets where
it is stored and from the `addrLookup` table.

> FIXME: is it possible that addresses with the same ID as the removed address,
> but with distinct network addresses, are kept in buckets of the address book?
> While they will not be accessible anymore, as there is no reference to them
> in the `addrLookup`, they will still be there.

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

The `loadFromFile` method, called when the address book is started, reads
address book entries from a file, passed to the address book constructor.
The file, at this point, does not need to exist.

The `saveRoutine` is started when the address book is started.
It saves the address book to the configured file every `dumpAddressInterval`,
hard-coded to 2 minutes.
It is also possible to save the content of the address book using the `Save`
method.
Saving the address book content to a file acquires the address book lock, also
employed by all other public methods.

## Implementation details

`AddrBook` is an interface, extending `service.Service`, declared in the `p2p/pex` package:

> // AddrBook is an address book used for tracking peers so we can gossip about them to others and select peers to dial.

Type `addrBook` is the (only) implementation of the `AddrBook` interface.

The address book stores peer addresses and information using `knownAddress` instances.
